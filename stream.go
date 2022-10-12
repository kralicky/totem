package totem

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
	gsync "github.com/kralicky/gpkg/sync"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"golang.org/x/net/context"
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
}

type StreamControllerOptions struct {
	name   string
	logger *zap.Logger
}

type StreamControllerOption func(*StreamControllerOptions)

func (o *StreamControllerOptions) apply(opts ...StreamControllerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithStreamName(name string) StreamControllerOption {
	return func(o *StreamControllerOptions) {
		o.name = name
	}
}

func WithLogger(logger *zap.Logger) StreamControllerOption {
	return func(o *StreamControllerOptions) {
		o.logger = logger
	}
}

// NewStreamHandler creates a new stream handler for the given stream and
// method set.
// There can be at most one stream handler per stream.
func NewStreamController(stream Stream, options ...StreamControllerOption) *StreamController {
	opts := StreamControllerOptions{}
	opts.apply(options...)
	if opts.logger == nil {
		opts.logger = Log.Named(opts.name)
	} else {
		opts.logger = opts.logger.Named(opts.name)
	}

	sh := &StreamController{
		StreamControllerOptions: opts,
		stream:                  stream,
		count:                   atomic.NewUint64(0),
		receiver:                newRecvWrapper(stream),
		uuid:                    uuid.New().String(),
	}
	desc, err := LoadServiceDesc(&ServerReflection_ServiceDesc)
	if err != nil {
		panic(err)
	}
	sh.RegisterServiceHandler(NewDefaultServiceHandler(stream.Context(),
		desc, newLocalServiceInvoker(sh, &ServerReflection_ServiceDesc, sh.logger, nil)))
	sh.RegisterServiceHandler(NewDefaultServiceHandler(stream.Context(),
		desc, newStreamControllerInvoker(sh, sh.logger)))
	return sh
}

func (sh *StreamController) RegisterServiceHandler(handler *ServiceHandler) {
	name := handler.Descriptor.GetName()
	sh.logger.With(
		zap.String("service", name),
	).Debug("registering service handler")

	list, _ := sh.services.LoadOrStore(name, &ServiceHandlerList{})
	list.Append(handler)
}

func (sh *StreamController) Request(ctx context.Context, m *RPC) <-chan *RPC {
	lg := sh.logger.With(
		zap.Uint64("tag", m.GetTag()),
		zap.String("service", m.GetServiceName()),
		zap.String("method", m.GetMethodName()),
		zap.String("type", fmt.Sprintf("%T", m.GetContent())),
	)
	lg.Debug("request")

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	otelgrpc.Inject(ctx, &md)

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
	lg := sh.logger.With(
		zap.Uint64("tag", tag),
	)
	lg.Debug("reply (success)")

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	otelgrpc.Inject(ctx, &md)

	sh.sendLock.Lock()
	err := sh.stream.Send(&RPC{
		Tag: tag,
		Content: &RPC_Response{
			Response: &Response{
				Response:    data,
				StatusProto: status.New(codes.OK, "").Proto(),
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
	lg := sh.logger.With(
		zap.Uint64("tag", tag),
		zap.Error(reply),
	)
	lg.Debug("reply")

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	otelgrpc.Inject(ctx, &md)

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

func (sh *StreamController) Kick(err error) {
	lg := sh.logger.With(
		zap.Error(err),
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

// Run will start the stream handler and block until the stream is finished.
// This function should only be called once.
func (sh *StreamController) Run(ctx context.Context) error {
	var streamErr error
	sh.receiver.Start()
	sh.logger.Debug("stream controller running")
	defer sh.logger.Debug("stream controller stopped")

	streamMetadata, hasStreamMetadata := metadata.FromIncomingContext(ctx)
	if hasStreamMetadata {
		// delete the traceparent header from stream metadata to avoid parenting
		// individual RPC traces to the stream trace
		streamMetadata.Delete("traceparent")
	}
	var inflightRequests sync.WaitGroup
	for {
		msg, err := sh.receiver.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				streamErr = err
			}
			break
		}
		md := msg.Metadata.ToMD()
		if hasStreamMetadata {
			md = metadata.Join(streamMetadata, md)
		}
		ctx := metadata.NewIncomingContext(ctx, md)

		b, sctx := otelgrpc.Extract(ctx, &md)
		ctx = trace.ContextWithSpanContext(baggage.ContextWithBaggage(ctx, b), sctx)
		msg.Metadata = nil

		switch msg.Content.(type) {
		case *RPC_Request:
			sh.logger.Debug("stream received RPC_Request")
			inflightRequests.Add(1)
			go func() {
				defer inflightRequests.Done()
				ctx, span := Tracer().Start(ctx, "RPC_Request: "+msg.QualifiedMethodName(),
					trace.WithAttributes(
						attribute.String("func", "streamController.Run/RPC_Request"),
						attribute.String("name", sh.name),
					),
				)
				defer span.End()
				// Received a request from the client
				svcName := msg.GetServiceName()
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
											_, _ = invoker.Invoke(addTotemToContext(ctx), proto.Clone(msg).(*RPC))
										} else {
											span.SetStatus(otelcodes.Error, fmt.Sprintf("method %q not found (broadcast)", method))
											err = status.Errorf(codes.NotFound, "method %q not found (broadcast)", method)
											return false
										}
										return true
									})
									if success {
										span.SetStatus(otelcodes.Ok, "")
										sh.Reply(ctx, msg.Tag, emptyResponse)
									} else {
										span.SetStatus(otelcodes.Error, err.Error())
										sh.ReplyErr(ctx, msg.Tag, err)
									}
									return
								}
							}
						}
						if invoker, ok := first.MethodInvokers[method]; ok {
							// Found a handler, call it
							// very important to clone the message here, otherwise the tag
							// will be overwritten, and we need to preserve it to reply to
							// the original request
							response, err := invoker.Invoke(addTotemToContext(ctx), proto.Clone(msg).(*RPC))
							if err != nil {
								span.SetStatus(otelcodes.Error, err.Error())
								sh.ReplyErr(ctx, msg.Tag, err)
								return
							}
							span.SetStatus(otelcodes.Ok, "")
							sh.Reply(ctx, msg.Tag, response)
						} else {
							span.SetStatus(otelcodes.Error, fmt.Sprintf("method %q not found", method))
							sh.ReplyErr(ctx, msg.Tag, status.Errorf(codes.NotFound, "method %q not found", method))
						}
						return
					}
				}

				// No handler found
				sh.logger.With(
					zap.String("service", svcName),
				).Debug("unknown service")
				span.SetStatus(otelcodes.Error, "unknown service "+svcName)
				sh.ReplyErr(ctx, msg.Tag, status.Error(codes.Unimplemented, "unknown service "+svcName))
			}()
		case *RPC_Response:
			sh.logger.Debug("stream received RPC_Response")
			// Received a response from the server
			future, ok := sh.pendingRPCs.LoadAndDelete(msg.Tag)
			if !ok {
				panic(fmt.Sprintf("fatal: unexpected tag: %d", msg.Tag))
			}
			future <- msg
			close(future)
		default:
			return fmt.Errorf("invalid content type")
		}
	}
	sh.pendingRPCs.Range(func(tag uint64, future chan *RPC) bool {
		sh.logger.With(
			zap.Uint64("tag", tag),
		).Debug("cancelling pending RPC")
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
	sh.logger.Debug("waiting for any inflight requests to complete")
	inflightRequests.Wait()
	sh.logger.Debug("all requests complete; stopping stream controller")
	return streamErr
}

func (sh *StreamController) ListServices(ctx context.Context, req *DiscoveryRequest) (*ServiceInfo, error) {
	sh.logger.With(
		zap.String("initiator", req.Initiator),
		zap.Any("visited", req.Visited),
		zap.Int("remainingHops", int(req.GetRemainingHops())),
	).Debug("ListServices")

	ctx, span := Tracer().Start(ctx, "streamController.ListServices",
		trace.WithAttributes(
			attribute.String("name", sh.name),
			attribute.String("initiator", req.Initiator),
			attribute.StringSlice("visited", req.Visited),
			attribute.Int("remainingHops", int(req.GetRemainingHops())),
		),
	)
	defer span.End()

	// Exact comparison to 0 is intentional; setting RemainingHops to a negative
	// number be used to disable the hop limit.
	if req.RemainingHops == 0 {
		span.AddEvent("DiscoveryStopped", trace.WithAttributes(
			attribute.String("reason", "hop limit reached"),
		))
		return &ServiceInfo{}, nil
	}
	if req.Initiator == sh.uuid {
		span.AddEvent("DiscoveryStopped", trace.WithAttributes(
			attribute.String("reason", "visited self"),
			attribute.String("id", sh.uuid),
		))
		return &ServiceInfo{}, nil
	}
	if slices.Contains(req.Visited, sh.uuid) {
		span.AddEvent("DiscoveryStopped", trace.WithAttributes(
			attribute.String("reason", "already visited this node"),
			attribute.String("id", sh.uuid),
		))
		return &ServiceInfo{}, nil
	}

	if req.Initiator == "" {
		req.Initiator = sh.uuid
	} else {
		req.Visited = append(req.Visited, sh.uuid)
	}
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
			method, ok := handler.MethodInvokers["ListServices"]
			if !ok {
				sh.logger.Warn("ServerReflection service does not have ListServices method")
				return true
			}
			ctx, span := Tracer().Start(ctx, "streamController.ListServices/Invoke",
				trace.WithAttributes(
					attribute.String("func", "streamController.ListServices/Invoke"),
					attribute.String("name", sh.name),
				),
			)
			defer span.End()
			reqBytes, _ := proto.Marshal(req)
			respData, err := method.Invoke(ctx, &RPC{
				ServiceName: "totem.ServerReflection",
				MethodName:  "ListServices",
				Content: &RPC_Request{
					Request: reqBytes,
				},
			})
			if err != nil {
				sh.logger.With(
					zap.Error(err),
				).Warn("error invoking ListServices")
				return true
			}
			remoteInfo := &ServiceInfo{}
			if err := proto.Unmarshal(respData, remoteInfo); err != nil {
				sh.logger.With(
					zap.Error(err),
				).Warn("error unmarshaling ListServices response")
				return true
			}
			span.AddEvent("Results", trace.WithAttributes(
				attribute.StringSlice("methods", remoteInfo.MethodNames()),
			))
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

	return &ServiceInfo{
		Services: maps.Values(deduped),
	}, nil
}

func (sh *StreamController) NewInvoker() MethodInvoker {
	return &streamControllerInvoker{
		controller: sh,
		logger:     sh.logger,
	}
}
