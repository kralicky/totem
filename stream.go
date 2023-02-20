package totem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/alitto/pond"
	"github.com/google/uuid"
	gsync "github.com/kralicky/gpkg/sync"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
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
}

type StreamControllerOptions struct {
	metrics  *MetricsExporter
	name     string
	logger   *zap.Logger
	wpParams WorkerPoolParameters
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

type WorkerPoolParameters struct {
	MaxWorkers       int
	MinWorkers       int
	MaxCapacity      int
	ResizingStrategy pond.ResizingStrategy
	IdleTimeout      time.Duration
}

func WithWorkerPoolParameters(params WorkerPoolParameters) StreamControllerOption {
	return func(o *StreamControllerOptions) {
		o.wpParams = params
	}
}

// Rx/Tx metrics are tracked in the following places:
// - For outgoing requests/incoming replies, in the clientconn.
// - For incoming requests/outgoing replies, in the localServiceInvoker.
func withMetricsExporter(exp *MetricsExporter) StreamControllerOption {
	return func(o *StreamControllerOptions) {
		o.metrics = exp
	}
}

// NewStreamController creates a new stream controller for the given stream and
// method set.
// There can be at most one stream controller per stream.
func NewStreamController(stream Stream, options ...StreamControllerOption) *StreamController {
	opts := StreamControllerOptions{
		wpParams: WorkerPoolParameters{
			MaxWorkers:       100,
			MaxCapacity:      1000,
			ResizingStrategy: pond.Eager(),
			IdleTimeout:      5 * time.Second,
		},
	}
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
		workerPool: pond.New(opts.wpParams.MaxWorkers, opts.wpParams.MaxCapacity,
			pond.Context(stream.Context()),
			pond.Strategy(opts.wpParams.ResizingStrategy),
			pond.MinWorkers(1),
			pond.IdleTimeout(opts.wpParams.IdleTimeout),
		),
	}
	desc, err := LoadServiceDesc(&ServerReflection_ServiceDesc)
	if err != nil {
		panic(err)
	}
	sh.RegisterServiceHandler(NewDefaultServiceHandler(stream.Context(),
		desc, newLocalServiceInvoker(sh, &ServerReflection_ServiceDesc, sh.logger, nil, sh.metrics)))
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
	serviceName := m.GetServiceName()
	methodName := m.GetMethodName()

	lg := sh.logger.With(
		zap.Uint64("tag", m.GetTag()),
		zap.String("service", serviceName),
		zap.String("method", methodName),
		zap.Strings("md", m.GetMetadata().Keys()),
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
	lg := sh.logger.With(
		zap.Uint64("tag", tag),
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

// Run will start the stream controller and block until the stream is finished.
// This function should only be called once.
func (sh *StreamController) Run(ctx context.Context) error {
	var streamErr error
	sh.receiver.Start()
	sh.logger.Debug("stream controller running")
	defer sh.logger.Debug("stream controller stopped")

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
			sh.logger.Debug("stream received RPC_Request")
			sh.workerPool.Submit(func() { sh.handleRequest(ctx, msg, md) })
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
	sh.workerPool.StopAndWait()
	sh.logger.Debug("all requests complete; stopping stream controller")
	return streamErr
}

func (sh *StreamController) ListServices(ctx context.Context, req *DiscoveryRequest) (*ServiceInfo, error) {
	sh.logger.With(
		zap.String("initiator", req.Initiator),
		zap.Any("visited", req.Visited),
		zap.Int("remainingHops", int(req.GetRemainingHops())),
	).Debug("ListServices")

	var span trace.Span
	if TracingEnabled {
		ctx, span = Tracer().Start(ctx, "streamController.ListServices",
			trace.WithAttributes(
				attribute.String("name", sh.name),
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
			var span trace.Span
			if TracingEnabled {
				ctx, span = Tracer().Start(ctx, "streamController.ListServices/Invoke",
					trace.WithAttributes(
						attribute.String("func", "streamController.ListServices/Invoke"),
						attribute.String("name", sh.name),
					),
				)
				defer span.End()
			}
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

func (sh *StreamController) handleRequest(ctx context.Context, msg *RPC, md metadata.MD) {
	var span trace.Span
	if TracingEnabled {
		ctx, span = Tracer().Start(ctx, "RPC_Request: "+msg.QualifiedMethodName(),
			trace.WithAttributes(
				attribute.String("func", "streamController.Run/RPC_Request"),
				attribute.String("name", sh.name),
			),
		)
		defer span.End()
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
								msg.Tag = requestTag // restore the tag
							} else {
								err := status.Errorf(codes.NotFound, "method %q not found (broadcast)", method)
								recordError(span, err)
								return false
							}
							return true
						})
						if success {
							recordSuccess(span)
							sh.Reply(ctx, msg.Tag, emptyResponse)
						} else {
							recordError(span, err)
							sh.ReplyErr(ctx, msg.Tag, err)
						}
						return
					}
				}
			}
			if invoker, ok := first.MethodInvokers[method]; ok {
				// Found a handler, call it
				// very important to copy the message here, otherwise the tag
				// will be overwritten, and we need to preserve it to reply to
				// the original request
				//  todo: does the above still apply?
				response, err := invoker.Invoke(ctx, msg)
				msg.Tag = requestTag // restore the tag
				if err != nil {
					recordError(span, err)
					sh.ReplyErr(ctx, msg.Tag, err)
					return
				}
				recordSuccess(span)
				sh.Reply(ctx, msg.Tag, response)
			} else {
				err := status.Errorf(codes.NotFound, "method %q not found", method)
				recordError(span, err)
				sh.ReplyErr(ctx, msg.Tag, err)
			}
			return
		}
	}

	// No handler found
	sh.logger.With(
		zap.String("service", svcName),
	).Debug("unknown service")
	err := status.Errorf(codes.Unimplemented, "unknown service: %q", svcName)
	recordError(span, err)
	sh.ReplyErr(ctx, msg.Tag, err)
}
