package totem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"slices"

	"github.com/alitto/pond"
	"github.com/google/uuid"
	gsync "github.com/kralicky/gpkg/sync"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/atomic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc/status"
)

type StreamController struct {
	UnimplementedServerReflectionServer
	StreamControllerOptions
	stream      Stream
	sendLock    sync.Mutex
	count       *atomic.Uint64
	pendingRPCs gsync.Map[uint64, chan *RPC]
	receiver    *recvWrapper
	kickOnce    sync.Once
	services    gsync.Map[string, *ServiceHandlerList]
	uuid        string
	workerPool  *pond.WorkerPool
	tracer      trace.Tracer
}

func DefaultWorkerPoolParams() WorkerPoolParameters {
	return WorkerPoolParameters{
		MaxWorkers:       100,
		MaxCapacity:      1000,
		ResizingStrategy: pond.Eager(),
		IdleTimeout:      5 * time.Second,
	}
}

type StreamControllerOptions struct {
	Metrics *MetricsExporter
	Name    string
	Logger  *slog.Logger

	// Rx/Tx metrics are tracked in the following places:
	// - For outgoing requests/incoming replies, in the clientconn.
	// - For incoming requests/outgoing replies, in the localServiceInvoker.
	WorkerPoolParams WorkerPoolParameters

	// TracerOptions should contain service name/namespace keys if multiple
	// services are running in the same process.
	TracerOptions []resource.Option

	BaseTopologyFlags TopologyFlags
}

type WorkerPoolParameters struct {
	MaxWorkers       int
	MinWorkers       int
	MaxCapacity      int
	ResizingStrategy pond.ResizingStrategy
	IdleTimeout      time.Duration
}

// NewStreamController creates a new stream controller for the given stream and
// method set.
// There can be at most one stream controller per stream.
func NewStreamController(stream Stream, opts StreamControllerOptions) *StreamController {
	if opts.Logger == nil {
		opts.Logger = Log.WithGroup(opts.Name)
	} else {
		opts.Logger = opts.Logger.WithGroup(opts.Name)
	}

	sh := &StreamController{
		StreamControllerOptions: opts,
		stream:                  stream,
		count:                   atomic.NewUint64(0),
		receiver:                newRecvWrapper(stream),
		uuid:                    uuid.New().String(),
		tracer:                  TracerProvider(opts.TracerOptions...).Tracer("totem"),
		workerPool: pond.New(opts.WorkerPoolParams.MaxWorkers, opts.WorkerPoolParams.MaxCapacity,
			pond.Context(stream.Context()),
			pond.Strategy(opts.WorkerPoolParams.ResizingStrategy),
			pond.MinWorkers(1),
			pond.IdleTimeout(opts.WorkerPoolParams.IdleTimeout),
		),
	}
	desc, err := LoadServiceDesc(&ServerReflection_ServiceDesc)
	if err != nil {
		panic(err)
	}
	sh.RegisterServiceHandler(NewDefaultServiceHandler(stream.Context(),
		desc, newLocalServiceInvoker(sh, &ServerReflection_ServiceDesc, sh.Logger, nil, sh.Metrics, opts.BaseTopologyFlags)))
	sh.RegisterServiceHandler(NewDefaultServiceHandler(stream.Context(), desc, sh.NewInvoker()))
	return sh
}

func (sh *StreamController) RegisterServiceHandler(handler *ServiceHandler) {
	name := handler.Descriptor.GetName()
	sh.Logger.Debug("registering service handler", "service", name)

	list, _ := sh.services.LoadOrStore(name, &ServiceHandlerList{})
	list.Append(handler)
}

func (sh *StreamController) Request(ctx context.Context, m *RPC) <-chan *RPC {
	serviceName := m.GetServiceName()
	methodName := m.GetMethodName()

	lg := sh.Logger.With(
		"tag", m.GetTag(),
		"service", serviceName,
		"method", methodName,
		"md", m.GetMetadata().Keys(),
	)
	lg.Debug("request")

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	if TracingEnabled {
		mdSupplier := metadataSupplier{&md}
		otel.GetTextMapPropagator().Inject(ctx, &mdSupplier)
	}

	m.Metadata = FromMD(md)

	tag := sh.count.Inc()
	m.Tag = tag
	ch := make(chan *RPC, 1)

	sh.pendingRPCs.Store(m.Tag, ch)

	sh.sendLock.Lock()
	err := sh.stream.Send(m)
	sh.sendLock.Unlock()

	if err != nil {
		if !errors.Is(err, io.EOF) {
			sh.Kick(err)
		}
	}
	return ch
}

func (sh *StreamController) Reply(ctx context.Context, tag uint64, data []byte) {
	lg := sh.Logger.With(
		"tag", tag,
	)
	lg.Debug("reply (success)")

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	if TracingEnabled {
		mdSupplier := metadataSupplier{&md}
		otel.GetTextMapPropagator().Inject(ctx, &mdSupplier)
	}

	sh.sendLock.Lock()
	err := sh.stream.Send(&RPC{
		Tag: tag,
		Content: &RPC_Response{
			Response: &Response{
				Response:    data,
				StatusProto: status.New(codes.OK, codes.OK.String()).Proto(),
			},
		},
		Metadata: FromMD(md),
	})
	sh.sendLock.Unlock()

	if err != nil {
		if !errors.Is(err, io.EOF) {
			sh.Kick(err)
		}
	}
}

func (sh *StreamController) ReplyErr(ctx context.Context, tag uint64, reply error) {
	lg := sh.Logger.With(
		"tag", tag,
		slog.Any("error", reply),
	)
	lg.Debug("reply")

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	if TracingEnabled {
		mdSupplier := metadataSupplier{&md}
		otel.GetTextMapPropagator().Inject(ctx, &mdSupplier)
	}

	sh.sendLock.Lock()
	err := sh.stream.Send(&RPC{
		Tag: tag,
		Content: &RPC_Response{
			Response: &Response{
				Response:    nil,
				StatusProto: status.Convert(reply).Proto(),
			},
		},
		Metadata: FromMD(md),
	})
	sh.sendLock.Unlock()

	if err != nil {
		if !errors.Is(err, io.EOF) {
			sh.Kick(err)
		}
	}
}

func (sh *StreamController) StreamReply(tag uint64, msg *ServerStreamMessage) {
	lg := sh.Logger.With(
		"tag", tag,
	)
	lg.Debug("reply (in stream)")

	sh.sendLock.Lock()
	err := sh.stream.Send(&RPC{
		Tag: tag,
		Content: &RPC_ServerStreamMsg{
			ServerStreamMsg: msg,
		},
	})
	sh.sendLock.Unlock()

	if err != nil {
		if !errors.Is(err, io.EOF) {
			sh.Kick(err)
		}
	}
}

func (sh *StreamController) Kick(err error) {
	lg := sh.Logger.With(
		"error", err,
	)
	sh.kickOnce.Do(func() {
		if status.Code(err) == codes.Canceled {
			lg.Debug("stream canceled; kicking")
		} else {
			lg.Warn("stream error; kicking")
		}
		sh.receiver.kick <- err
	})
}

// If the stream is a client stream, this will call CloseSend on the stream.
// If the stream is a server stream, this will call Recv on the stream until
// it returns io.EOF.
func (sh *StreamController) CloseOrRecv() error {
	sh.sendLock.Lock()
	defer sh.sendLock.Unlock()
	switch stream := sh.stream.(type) {
	case ClientStream:
		if err := stream.CloseSend(); err != nil {
			return err
		}
		sh.receiver.Wait()
		return nil
	case ServerStream:
		sh.receiver.Wait()
		for {
			_, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}
	}
	return nil
}

var (
	emptyResponse []byte
)

func init() {
	data, err := proto.Marshal(&emptypb.Empty{})
	if err != nil {
		panic(err)
	}
	emptyResponse = data
}

// Run will start the stream controller and block until the stream is finished.
// This function should only be called once.
func (sh *StreamController) Run(ctx context.Context) error {
	var streamErr error
	sh.receiver.Start()
	sh.Logger.Debug("stream controller running")
	defer sh.Logger.Debug("stream controller stopped")

	for {
		msg, err := sh.receiver.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				streamErr = err
			}
			break
		}

		md := msg.Metadata.ToMD()
		ctx := metadata.NewIncomingContext(ctx, md)

		if TracingEnabled {
			mdSupplier := metadataSupplier{&md}
			ctx = otel.GetTextMapPropagator().Extract(ctx, &mdSupplier)
		}

		switch msg.Content.(type) {
		case *RPC_Request:
			sh.Logger.Debug("stream received RPC_Request")
			sh.workerPool.Submit(func() { sh.handleRequest(ctx, msg, md) })
		case *RPC_Response:
			sh.Logger.Debug("stream received RPC_Response")
			// Received a response from the server

			future, ok := sh.pendingRPCs.LoadAndDelete(msg.Tag)
			if !ok {
				panic(fmt.Sprintf("fatal: unexpected tag: %d", msg.Tag))
			}
			future <- msg

			close(future)
		case *RPC_ServerStreamMsg:
			sh.Logger.Debug("stream received RPC_ServerStreamMsg")
			future, ok := sh.pendingRPCs.Load(msg.Tag)
			if !ok {
				panic(fmt.Sprintf("fatal: unexpected tag: %d", msg.Tag))
			}
			future <- msg
			// leave the channel open, a message of this type is non-terminating
		default:
			return fmt.Errorf("invalid content type")
		}
	}
	sh.pendingRPCs.Range(func(tag uint64, future chan *RPC) bool {
		sh.Logger.Debug("cancelling pending RPC", "tag", tag)

		future <- &RPC{
			Tag: tag,
			Content: &RPC_Response{
				Response: &Response{
					StatusProto: status.Convert(streamErr).Proto(),
				},
			},
		}
		close(future)
		sh.pendingRPCs.Delete(tag)
		return true
	})
	sh.Logger.Debug("waiting for any inflight requests to complete")
	sh.workerPool.StopAndWait()
	sh.Logger.Debug("all requests complete; stopping stream controller")
	return streamErr
}

func (sh *StreamController) ListServices(ctx context.Context, req *DiscoveryRequest) (*ServiceInfo, error) {
	sh.Logger.Debug("ListServices", "initiator", req.Initiator,
		"visited", req.Visited,
		"remainingHops", int(req.GetRemainingHops()))

	var span trace.Span
	if TracingEnabled {
		ctx, span = sh.tracer.Start(ctx, "ListServices",
			trace.WithAttributes(
				attribute.String("name", sh.Name),
				attribute.String("initiator", req.Initiator),
				attribute.StringSlice("visited", req.Visited),
				attribute.Int("remainingHops", int(req.GetRemainingHops())),
			),
		)
		defer span.End()
	}

	// Exact comparison to 0 is intentional; setting RemainingHops to a negative
	// number be used to disable the hop limit.
	if req.RemainingHops == 0 {
		if TracingEnabled {
			span.AddEvent("DiscoveryStopped", trace.WithAttributes(
				attribute.String("reason", "hop limit reached"),
			))
		}
		return &ServiceInfo{}, nil
	}
	if req.Initiator == sh.uuid {
		if TracingEnabled {
			span.AddEvent("DiscoveryStopped", trace.WithAttributes(
				attribute.String("reason", "visited self"),
				attribute.String("id", sh.uuid),
			))
		}
		return &ServiceInfo{}, nil
	}
	if slices.Contains(req.Visited, sh.uuid) {
		if TracingEnabled {
			span.AddEvent("DiscoveryStopped", trace.WithAttributes(
				attribute.String("reason", "already visited this node"),
				attribute.String("id", sh.uuid),
			))
		}
		return &ServiceInfo{}, nil
	}

	req.Visited = append(req.Visited, sh.uuid)
	req.RemainingHops--

	var services []*descriptorpb.ServiceDescriptorProto
	sh.services.Range(func(key string, value *ServiceHandlerList) bool {
		value.Range(func(sh *ServiceHandler) bool {
			services = append(services, proto.Clone(sh.Descriptor).(*descriptorpb.ServiceDescriptorProto))
			return false
		})
		return true
	})

	if list, ok := sh.services.Load("totem.ServerReflection"); ok {
		list.Range(func(handler *ServiceHandler) bool {
			invoker, ok := handler.MethodInvokers["ListServices"]
			if !ok {
				sh.Logger.Warn("ServerReflection service does not have ListServices method")
				return true
			}
			invokerFlags := invoker.TopologyFlags()

			var attributes []attribute.KeyValue
			if TracingEnabled {
				attributes = []attribute.KeyValue{
					attribute.String("id", sh.uuid),
					attribute.String("name", sh.Name),
					attribute.String("initiator", req.Initiator),
					attribute.StringSlice("visited", req.Visited),
					attribute.Int("remainingHops", int(req.GetRemainingHops())),
					attribute.String("handler", handler.Descriptor.GetName()),
					attribute.String("handlerFlags", handler.TopologyFlags.DisplayName()),
					attribute.String("invokerFlags", invokerFlags.DisplayName()),
				}
			}

			if invokerFlags&TopologyLocal != 0 {
				if TracingEnabled {
					span.AddEvent("Skipping local", trace.WithAttributes(attributes...))
				}
				return true
			}

			// Skip if this is the same stream we received the request on, unless
			// the request came from a stream spliced to this controller. In that case,
			// because we are acting on behalf of the spliced client, our peer
			// should not be treated as its direct peer.
			// We can determine if the sender is one of our spliced clients by
			// checking the topology flags. The controller context for spliced streams
			// will be the same.
			if invokerFlags&TopologySpliced == 0 ||
				handler.controllerContext == sh.stream.Context() {
				if TracingEnabled {
					span.AddEvent("Skipping peer", trace.WithAttributes(attributes...))
				}
				return true
			}

			if TracingEnabled {
				span.AddEvent("Sending ListServices", trace.WithAttributes(attributes...))
			}

			reqBytes, _ := proto.Marshal(req)
			respData, err := invoker.Invoke(ctx, &RPC{
				ServiceName: "totem.ServerReflection",
				MethodName:  "ListServices",
				Content: &RPC_Request{
					Request: reqBytes,
				},
			})
			if err != nil {
				sh.Logger.Warn("error invoking ListServices", "error", err)

				return true
			}
			remoteInfo := &ServiceInfo{}
			if err := proto.Unmarshal(respData, remoteInfo); err != nil {
				sh.Logger.Warn("error unmarshaling ListServices response", "error", err)

				return true
			}
			if TracingEnabled {
				span.AddEvent("Results", trace.WithAttributes(
					attribute.StringSlice("methods", remoteInfo.MethodNames()),
				))
			}
			services = append(services, remoteInfo.Services...)
			return true
		})
	}

	// remove duplicates
	deduped := make(map[string]*descriptorpb.ServiceDescriptorProto)
	for _, svc := range services {
		if _, ok := deduped[svc.GetName()]; !ok {
			deduped[svc.GetName()] = svc
		}
	}

	if TracingEnabled {
		keys := make([]string, 0, len(deduped))
		for k := range deduped {
			keys = append(keys, k)
		}
		span.AddEvent("Added Services", trace.WithAttributes(
			attribute.StringSlice("services", keys),
		))
	}
	values := []*descriptorpb.ServiceDescriptorProto{}
	for _, svc := range deduped {
		if TracingEnabled {
		}
		values = append(values, svc)
	}
	return &ServiceInfo{
		Services: values,
	}, nil
}

func (sh *StreamController) NewInvoker() *streamControllerInvoker {
	return newStreamControllerInvoker(sh, sh.BaseTopologyFlags, sh.Logger)
}

func (sh *StreamController) handleRequest(ctx context.Context, msg *RPC, md metadata.MD) {
	span := trace.SpanFromContext(ctx)
	if TracingEnabled {
		span.AddEvent("Request: " + msg.QualifiedMethodName())
	}
	// Received a request from the client
	svcName := msg.GetServiceName()
	requestTag := msg.Tag
	if handlers, ok := sh.services.Load(svcName); ok && handlers.Len() > 0 {
		if first := handlers.First(); first != nil {
			method := msg.GetMethodName()
			if handlers.Len() > 1 {
				if qos, ok := first.MethodQOS[method]; ok {
					switch qos.ReplicationStrategy {
					case ReplicationStrategy_First:
						// continue below
					case ReplicationStrategy_Broadcast:
						var err error
						success := handlers.Range(func(sh *ServiceHandler) bool {
							if invoker, ok := sh.MethodInvokers[method]; ok {
								_, _ = invoker.Invoke(ctx, msg)
							} else {
								err := status.Errorf(codes.NotFound, "method %q not found (broadcast)", method)
								recordError(span, err)
								return false
							}
							return true
						})
						if success {
							recordSuccess(span)
							sh.Reply(ctx, requestTag, emptyResponse)
						} else {
							recordError(span, err)
							sh.ReplyErr(ctx, requestTag, err)
						}
						return
					}
				}
			}
			if invoker, ok := first.MethodInvokers[method]; ok {
				// Found a handler, call it
				info := first.MethodInfo[method]
				switch {
				case !info.IsServerStream && !info.IsClientStream:
					// very important to copy the message here, otherwise the tag
					// will be overwritten, and we need to preserve it to reply to
					// the original request
					//  todo: does the above still apply?
					response, err := invoker.Invoke(ctx, msg)
					if err != nil {
						recordError(span, err)
						sh.ReplyErr(ctx, requestTag, err)
						return
					}
					recordSuccess(span)
					sh.Reply(ctx, requestTag, response)
				case info.IsServerStream && !info.IsClientStream:
					responseCC := make(chan chan<- *RPC, 1)
					responseC := make(chan *RPC, 1)
					responseCC <- responseC
					err := invoker.InvokeStream(ctx, msg, responseCC)
					if err != nil {
						recordError(span, err)
						sh.ReplyErr(ctx, requestTag, err)
						return
					}
					var exitResponse *RPC
					for {
						select {
						case <-ctx.Done():
							recordError(span, ctx.Err())
							return
						case response, ok := <-responseC:
							if !ok {
								// stream closed
								if exitResponse != nil {
									stat := exitResponse.GetResponse().GetStatus()
									if stat.Code() != codes.OK {
										recordError(span, stat.Err())
										// replace any outgoing metadata with the trailers from the last response
										sh.ReplyErr(metadata.NewOutgoingContext(ctx, exitResponse.GetMetadata().ToMD()), requestTag, stat.Err())
										return
									}
									recordSuccess(span)
									sh.Reply(metadata.NewOutgoingContext(ctx, exitResponse.GetMetadata().ToMD()), requestTag, exitResponse.GetResponse().GetResponse())
								}
								return
							}
							switch response.Content.(type) {
							case *RPC_Response:
								exitResponse = response
								// continue and wait for responseC to be closed by the handler
							case *RPC_ServerStreamMsg:
								sh.StreamReply(requestTag, response.GetServerStreamMsg())
							}
						}
					}
				}
			} else {
				err := status.Errorf(codes.NotFound, "method %q not found", method)
				recordError(span, err)
				sh.ReplyErr(ctx, requestTag, err)
			}
			return
		}
	}

	// No handler found
	sh.Logger.Debug("unknown service", "service", svcName)

	err := status.Errorf(codes.Unimplemented, "unknown service: %q", svcName)
	recordError(span, err)
	sh.ReplyErr(ctx, requestTag, err)
}
