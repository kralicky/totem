package totem

import (
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type SharedMethodSet struct {
	methodsLock sync.RWMutex
	methods     map[string]MethodInvoker
}

func NewSharedMethodSet() *SharedMethodSet {
	return &SharedMethodSet{
		methods: make(map[string]MethodInvoker),
	}
}

func (s *SharedMethodSet) Put(name string, invoker MethodInvoker) {
	s.methodsLock.Lock()
	defer s.methodsLock.Unlock()
	s.methods[name] = invoker
}

func (s *SharedMethodSet) Get(name string) (MethodInvoker, bool) {
	s.methodsLock.RLock()
	defer s.methodsLock.RUnlock()
	m, ok := s.methods[name]
	return m, ok
}

func (s *SharedMethodSet) Range(fn func(name string, invoker MethodInvoker) bool) {
	s.methodsLock.RLock()
	defer s.methodsLock.RUnlock()
	for name, invoker := range s.methods {
		if !fn(name, invoker) {
			return
		}
	}
}

type MethodInvoker interface {
	Invoke(ctx context.Context, rpc *RPC) ([]byte, error)
}

type localServiceInvoker struct {
	serviceImpl interface{}
	service     *grpc.ServiceDesc
	methods     map[string]grpc.MethodDesc
	logger      *zap.Logger
	interceptor grpc.UnaryServerInterceptor
}

func newLocalServiceInvoker(
	serviceImpl interface{},
	service *grpc.ServiceDesc,
	logger *zap.Logger,
	interceptor grpc.UnaryServerInterceptor,
) *localServiceInvoker {
	handlers := make(map[string]grpc.MethodDesc)
	for _, method := range service.Methods {
		handlers[method.MethodName] = method
	}
	return &localServiceInvoker{
		serviceImpl: serviceImpl,
		service:     service,
		methods:     handlers,
		logger:      logger,
		interceptor: interceptor,
	}
}

func (l *localServiceInvoker) Invoke(ctx context.Context, req *RPC) ([]byte, error) {
	l.logger.With(
		zap.String("service", req.GetServiceName()),
		zap.String("method", req.GetMethodName()),
		zap.Uint64("tag", req.GetTag()),
	).Debug("invoking method using local service")

	attrs := []attribute.KeyValue{attribute.String("func", "localServiceInvoker.Invoke")}

	{
		// md only introspected here for tracing purposes
		md, _ := metadata.FromIncomingContext(ctx)
		attrs = append(attrs, FromMD(md).KV()...)
	}

	ctx, span := Tracer().Start(ctx, "Invoke/Local: "+req.QualifiedMethodName(),
		trace.WithAttributes(attrs...))
	defer span.End()

	if m, ok := l.methods[req.MethodName]; ok {
		resp, err := m.Handler(l.serviceImpl, addTotemToContext(ctx), func(v any) error {
			return proto.Unmarshal(req.GetRequest(), v.(proto.Message))
		}, l.interceptor)
		if err != nil {
			recordError(span, err)
			return nil, err
		}
		recordSuccess(span)
		return proto.Marshal(resp.(proto.Message))
	} else {
		err := status.Errorf(codes.Unimplemented, "unknown method %s", req.MethodName)
		recordError(span, err)
		return nil, err
	}
}

type streamControllerInvoker struct {
	controller *streamController
	logger     *zap.Logger
}

func newStreamControllerInvoker(ctrl *streamController, logger *zap.Logger) *streamControllerInvoker {
	return &streamControllerInvoker{
		controller: ctrl,
		logger:     logger,
	}
}

func (r *streamControllerInvoker) Invoke(ctx context.Context, req *RPC) ([]byte, error) {
	r.logger.With(
		zap.String("service", req.GetServiceName()),
		zap.String("method", req.GetMethodName()),
		zap.Uint64("tag", req.GetTag()),
	).Debug("invoking method using stream controller")

	// convert the incoming context to an outgoing context
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	attrs := []attribute.KeyValue{
		attribute.String("func", "streamControllerInvoker.Invoke"),
		attribute.String("name", r.controller.name),
	}
	attrs = append(attrs, FromMD(md).KV()...)
	ctx, span := Tracer().Start(ctx, "Invoke/Stream: "+req.QualifiedMethodName(),
		trace.WithAttributes(attrs...))
	defer span.End()

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

		return resp.GetResponse(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
