package totem

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

type MethodInvoker interface {
	Invoke(ctx context.Context, rpc *RPC) ([]byte, error)
	InvokeStream(ctx context.Context, rpc *RPC, wc <-chan chan<- *RPC) error
	TopologyFlags() TopologyFlags
}

type localServiceInvoker struct {
	serviceImpl interface{}
	service     *grpc.ServiceDesc
	methods     map[string]grpc.MethodDesc
	streams     map[string]grpc.StreamDesc
	logger      *zap.Logger
	interceptor grpc.UnaryServerInterceptor
	metrics     *MetricsExporter
	flags       TopologyFlags
}

func newLocalServiceInvoker(
	serviceImpl interface{},
	service *grpc.ServiceDesc,
	logger *zap.Logger,
	interceptor grpc.UnaryServerInterceptor,
	metrics *MetricsExporter,
	baseTopologyFlags TopologyFlags,
) *localServiceInvoker {
	methods := make(map[string]grpc.MethodDesc)
	streams := make(map[string]grpc.StreamDesc)
	for _, method := range service.Methods {
		methods[method.MethodName] = method
	}
	for _, stream := range service.Streams {
		streams[stream.StreamName] = stream
	}
	return &localServiceInvoker{
		serviceImpl: serviceImpl,
		service:     service,
		methods:     methods,
		streams:     streams,
		logger:      logger,
		interceptor: interceptor,
		metrics:     metrics,
		flags:       baseTopologyFlags | TopologyLocal,
	}
}

func (l *localServiceInvoker) Invoke(ctx context.Context, req *RPC) ([]byte, error) {
	serviceName := req.GetServiceName()
	methodName := req.GetMethodName()

	l.logger.With(
		zap.String("service", serviceName),
		zap.String("method", methodName),
		zap.Uint64("tag", req.GetTag()),
		zap.Strings("md", req.GetMetadata().Keys()),
	).Debug("invoking method using local service")

	span := trace.SpanFromContext(ctx)

	if m, ok := l.methods[req.MethodName]; ok {
		startTime := time.Now()
		resp, err := m.Handler(l.serviceImpl, ctx, func(v any) error {
			reqBytes := req.GetRequest()
			span.AddEvent("Invoke/Local: "+req.QualifiedMethodName(),
				trace.WithAttributes(attribute.Int("size", len(reqBytes))))
			l.metrics.TrackRxBytes(serviceName, methodName, int64(len(reqBytes)))
			return proto.Unmarshal(reqBytes, protoimpl.X.ProtoMessageV2Of(v))
		}, l.interceptor)
		if err != nil {
			recordError(span, err)
			return nil, err
		}
		respBytes, err := proto.Marshal(protoimpl.X.ProtoMessageV2Of(resp))
		if err != nil {
			recordError(span, err)
			return nil, err
		}
		recordSuccess(span)
		l.metrics.TrackSvcRxLatency(serviceName, methodName, time.Since(startTime))
		l.metrics.TrackTxBytes(serviceName, methodName, int64(len(respBytes)))
		return respBytes, nil
	} else {
		err := status.Errorf(codes.Unimplemented, "unknown method %s", req.MethodName)
		recordError(span, err)
		return nil, err
	}
}

func (l *localServiceInvoker) InvokeStream(ctx context.Context, req *RPC, wc <-chan chan<- *RPC) error {
	writer := <-wc
	// serviceName := req.GetServiceName()
	methodName := req.GetMethodName()
	s, ok := l.streams[methodName]
	if !ok {
		return status.Errorf(codes.Unimplemented, "unknown method %s", methodName)
	}
	go func() {
		defer close(writer)
		streamWrapper := newLocalServerStreamWrapper(ctx, req, writer)
		rpcError := s.Handler(l.serviceImpl, streamWrapper)
		var stat *status.Status
		if rpcError != nil {
			stat = status.Convert(rpcError)
		} else {
			stat = status.New(codes.OK, "")
		}
		trailers := streamWrapper.Close()
		response := &RPC{
			Content: &RPC_Response{
				Response: &Response{
					StatusProto: stat.Proto(),
				},
			},
			Metadata: FromMD(trailers),
		}
		writer <- response
	}()
	return nil
}

func (l *localServiceInvoker) TopologyFlags() TopologyFlags {
	return l.flags
}

type streamControllerInvoker struct {
	controller *StreamController
	logger     *zap.Logger
	flags      TopologyFlags
}

func newStreamControllerInvoker(ctrl *StreamController, flags TopologyFlags, logger *zap.Logger) *streamControllerInvoker {
	return &streamControllerInvoker{
		controller: ctrl,
		logger:     logger,
		flags:      flags,
	}
}

func (r *streamControllerInvoker) Invoke(ctx context.Context, req *RPC) ([]byte, error) {
	serviceName := req.GetServiceName()
	methodName := req.GetMethodName()
	r.logger.With(
		zap.String("service", serviceName),
		zap.String("method", methodName),
		zap.Uint64("tag", req.GetTag()),
		zap.Strings("md", req.GetMetadata().Keys()),
	).Debug("invoking method using stream controller")

	// convert the incoming context to an outgoing context
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	reqSize := len(req.GetRequest())

	span := trace.SpanFromContext(ctx)
	span.AddEvent("Stream Request: "+req.QualifiedMethodName(),
		trace.WithAttributes(attribute.Int("size", reqSize)))

	r.controller.Metrics.TrackTxBytes(serviceName, methodName, int64(reqSize))
	rc := r.controller.Request(ctx, req)
	select {
	case rpc := <-rc:
		resp := rpc.GetResponse()
		stat := resp.GetStatus()
		if err := stat.Err(); err != nil {
			recordErrorStatus(span, stat)
			return nil, err
		}
		recordSuccess(span)
		span.AddEvent("Stream Response: "+req.QualifiedMethodName(),
			trace.WithAttributes(
				attribute.Int("size", len(resp.Response)),
				attribute.Int("code", int(stat.Code())),
			))
		r.controller.Metrics.TrackRxBytes(serviceName, methodName, int64(len(resp.Response)))
		return resp.GetResponse(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *streamControllerInvoker) InvokeStream(ctx context.Context, req *RPC, wc <-chan chan<- *RPC) error {

	serviceName := req.GetServiceName()
	methodName := req.GetMethodName()
	r.logger.With(
		zap.String("service", serviceName),
		zap.String("method", methodName),
		zap.Uint64("tag", req.GetTag()),
		zap.Strings("md", req.GetMetadata().Keys()),
	).Debug("invoking stream using stream controller")

	// convert the incoming context to an outgoing context
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	reqSize := len(req.GetRequest())

	span := trace.SpanFromContext(ctx)
	span.AddEvent("Stream Request: "+req.QualifiedMethodName(),
		trace.WithAttributes(attribute.Int("size", reqSize)))

	r.controller.Metrics.TrackTxBytes(serviceName, methodName, int64(reqSize))

	writer := <-wc
	go func() {
		defer close(writer)

		rc := r.controller.Request(ctx, req)
		for {
			select {
			case rpc := <-rc:
				switch rpc.Content.(type) {
				case *RPC_ServerStreamMsg:
					writer <- rpc
				case *RPC_Response:
					writer <- rpc
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (r *streamControllerInvoker) TopologyFlags() TopologyFlags {
	return r.flags
}
