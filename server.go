package totem

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	ServerOptions
	stream             Stream
	lock               sync.RWMutex
	controller         *StreamController
	logger             *zap.Logger
	splicedControllers []*StreamController
}

type ServerOptions struct {
	name              string
	interceptors      InterceptorConfig
	metrics           *MetricsExporter
	discoveryHopLimit int32
}

type InterceptorConfig struct {
	// This interceptor functions similarly to a standard unary server interceptor,
	// and will be called for RPCs that are about to be invoked locally. When
	// an RPC is passed through to a spliced stream, this interceptor will not
	// be called.
	Incoming grpc.UnaryServerInterceptor

	// This interceptor functions similarly to a standard unary client interceptor,
	// with the one caveat that the [grpc.ClientConn] passed to the interceptor
	// will always be nil, and must not be used. The interceptor should still
	// forward the nil argument to the invoker for potential forward compatibility.
	// This interceptor is not called for RPCs being passed through to a spliced
	// stream.
	Outgoing grpc.UnaryClientInterceptor
}

func WithInterceptors(config InterceptorConfig) ServerOption {
	return func(o *ServerOptions) {
		o.interceptors = config
	}
}

func WithMetrics(provider *metric.MeterProvider, staticAttrs ...attribute.KeyValue) ServerOption {
	return func(o *ServerOptions) {
		o.metrics = NewMetricsExporter(provider, staticAttrs...)
	}
}

func WithDiscoveryHopLimit(limit int32) ServerOption {
	return func(o *ServerOptions) {
		o.discoveryHopLimit = limit
	}
}

type ServerOption func(*ServerOptions)

func (o *ServerOptions) apply(opts ...ServerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithName(name string) ServerOption {
	return func(o *ServerOptions) {
		o.name = name
	}
}

func NewServer(stream Stream, opts ...ServerOption) (*Server, error) {
	options := ServerOptions{
		discoveryHopLimit: -1,
	}
	options.apply(opts...)

	lg := Log.Named(options.name)

	ctrl := NewStreamController(stream, WithStreamName(options.name), withMetricsExporter(options.metrics))

	srv := &Server{
		ServerOptions: options,
		stream:        stream,
		controller:    ctrl,
		logger:        lg,
	}

	return srv, nil
}

// Implements grpc.ServiceRegistrar
func (r *Server) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	if impl != nil {
		ht := reflect.TypeOf(desc.HandlerType).Elem()
		st := reflect.TypeOf(impl)
		if !st.Implements(ht) {
			log.Fatalf("grpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
		}
		r.register(desc, impl)
	} else {
		log.Fatalf("grpc: Server.RegisterService found nil service implementation")
	}
}

// Splice configures this server to forward any incoming RPCs for the given
// service(s) to a different totem stream.
// The totem server will handle closing the spliced stream.
func (r *Server) Splice(stream Stream, opts ...StreamControllerOption) error {
	ctrlOptions := StreamControllerOptions{}
	ctrlOptions.apply(opts...)
	name := "spliced"
	if ctrlOptions.name != "" {
		name = r.name + "->" + ctrlOptions.name
	}
	ctrlOptions.name = name
	lg := r.logger.Named(name)

	r.lock.Lock()
	defer r.lock.Unlock()

	ctrl := NewStreamController(stream, WithLogger(lg), WithStreamName(name))

	r.controller.services.Range(func(key string, value *ServiceHandlerList) bool {
		value.Range(func(sh *ServiceHandler) bool {
			if !sh.IsLocal {
				return true
			}
			if proto.HasExtension(sh.Descriptor.Options, E_Visibility) {
				vis := proto.GetExtension(sh.Descriptor.Options, E_Visibility).(*Visibility)
				if vis.SplicedClients {
					r.logger.With(
						zap.String("service", sh.Descriptor.GetName()),
					).Debug("enabling local service on spliced controller due to visibility option")
					ctrl.RegisterServiceHandler(sh)
				}
			}
			return true
		})
		return true
	})

	go func() {
		if err := ctrl.Run(stream.Context()); err != nil {
			if status.Code(err) == codes.Canceled {
				r.logger.Debug("stream closed")
			} else {
				r.logger.With(
					zap.Error(err),
				).Warn("stream exited with error")
			}
		}
	}()

	ctx := stream.Context()
	var span trace.Span
	if TracingEnabled {
		ctx, span = Tracer().Start(ctx, "Server.Splice/Discovery",
			trace.WithAttributes(
				attribute.String("name", r.name),
			),
		)
	}
	info, err := discoverServices(ctx, ctrl, r.discoveryHopLimit)
	if err != nil {
		err := fmt.Errorf("service discovery failed: %w", err)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return err
	}
	if TracingEnabled {
		span.End()
	}
	r.logger.With(
		zap.Any("methods", info.MethodNames()),
	).Debug("splicing stream")

	handlerInvoker := ctrl.NewInvoker()
	for _, desc := range info.Services {
		r.controller.RegisterServiceHandler(NewDefaultServiceHandler(stream.Context(), desc, handlerInvoker))
	}

	// these are tracked so that when the server starts and runs discovery for
	// the main controller, discovered service handlers can be replicated to
	// spliced clients
	r.splicedControllers = append(r.splicedControllers, ctrl)

	return nil
}

func (r *Server) register(serviceDesc *grpc.ServiceDesc, impl interface{}) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.logger.With(
		zap.String("service", serviceDesc.ServiceName),
	).Debug("registering service")

	reflectionDesc, err := LoadServiceDesc(serviceDesc)
	if err != nil {
		log.Fatalf("totem: failed to load service descriptor: %v", err)
	}

	r.controller.RegisterServiceHandler(NewDefaultServiceHandler(r.Context(), reflectionDesc,
		newLocalServiceInvoker(impl, serviceDesc, r.logger, r.interceptors.Incoming, r.metrics)))
}

// Serve starts the totem server, which takes control of the stream and begins
// handling incoming and outgoing RPCs.
func (r *Server) Serve() (grpc.ClientConnInterface, <-chan error) {
	r.logger.Debug("starting totem server")
	r.lock.RLock()
	ch := make(chan error, 2)

	go func() {
		runErr := r.controller.Run(r.Context())
		if runErr != nil {
			if status.Code(runErr) == codes.Canceled {
				r.logger.With(
					zap.Error(runErr),
				).Debug("stream canceled")
			} else {
				r.logger.With(
					zap.Error(runErr),
				).Warn("stream exited with error")
			}
		} else {
			r.logger.Debug("stream closed")
		}
		r.lock.RUnlock()

		r.lock.Lock()
		defer r.lock.Unlock()
		if runErr != nil {
			r.logger.With(
				zap.Error(runErr),
				zap.Int("numControllers", len(r.splicedControllers)),
			).Debug("kicking spliced controllers")
		}
		for _, spliced := range r.splicedControllers {
			if runErr != nil {
				spliced.Kick(runErr)
			}
			spliced.CloseOrRecv()
		}
		r.controller.CloseOrRecv()
		ch <- runErr
	}()

	ctx := context.Background()
	var span trace.Span
	if TracingEnabled {
		ctx, span = Tracer().Start(r.Context(), "Server.Serve/Discovery",
			trace.WithAttributes(
				attribute.String("name", r.name),
			),
		)
	}
	info, err := discoverServices(ctx, r.controller, r.discoveryHopLimit)
	if TracingEnabled {
		span.End()
	}

	if err != nil {
		r.controller.Kick(fmt.Errorf("service discovery failed: %w", err))
		r.controller.CloseOrRecv()
		ch <- err
		return nil, ch
	}
	r.logger.With(
		zap.Any("methods", info.MethodNames()),
	).Debug("service discovery complete")

	invoker := r.controller.NewInvoker()
	for _, svcDesc := range info.Services {
		for _, ctrl := range r.splicedControllers {
			ctrl.RegisterServiceHandler(NewDefaultServiceHandler(r.Context(), svcDesc, invoker))
		}
	}

	return &ClientConn{
		controller:  r.controller,
		interceptor: r.interceptors.Outgoing,
		tracer:      Tracer(),
		logger:      r.logger.Named("cc"),
		metrics:     r.metrics,
	}, ch
}

// Returns the server's stream context. Only valid after Serve has been called.
func (r *Server) Context() context.Context {
	return r.stream.Context()
}
