package totem

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
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
	tracer             trace.Tracer

	setupCtx  context.Context
	setupSpan trace.Span
}

type ServerOptions struct {
	name              string
	discoveryHopLimit int32
	interceptors      InterceptorConfig
	metrics           *MetricsExporter
	tracerOptions     []resource.Option
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

func WithTracerOptions(opts ...resource.Option) ServerOption {
	return func(o *ServerOptions) {
		o.tracerOptions = opts
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

	ctrl := NewStreamController(stream, StreamControllerOptions{
		Logger:           lg,
		Name:             options.name,
		Metrics:          options.metrics,
		WorkerPoolParams: DefaultWorkerPoolParams(),
		TracerOptions:    options.tracerOptions,
	})

	tracer := TracerProvider(options.tracerOptions...).Tracer(TracerName)
	setupCtx, span := tracer.Start(stream.Context(), "Stream Initialization",
		trace.WithAttributes(attribute.String("name", options.name)),
		trace.WithNewRoot(),
	)

	srv := &Server{
		ServerOptions: options,
		stream:        stream,
		controller:    ctrl,
		logger:        lg,
		setupCtx:      setupCtx,
		setupSpan:     span,
		tracer:        tracer,
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
func (r *Server) Splice(stream Stream, opts ...ServerOption) error {
	options := ServerOptions{
		name:              r.name,
		discoveryHopLimit: r.discoveryHopLimit,
		metrics:           r.metrics,
		tracerOptions:     r.tracerOptions,
	}
	options.apply(opts...)

	r.lock.Lock()
	defer r.lock.Unlock()

	ctrl := NewStreamController(stream, StreamControllerOptions{
		Logger:            r.logger,
		Name:              options.name,
		Metrics:           options.metrics,
		WorkerPoolParams:  DefaultWorkerPoolParams(),
		TracerOptions:     options.tracerOptions,
		BaseTopologyFlags: TopologySpliced,
	})

	r.controller.services.Range(func(key string, value *ServiceHandlerList) bool {
		value.Range(func(sh *ServiceHandler) bool {
			if sh.TopologyFlags&TopologyLocal == 0 {
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

	info, err := discoverServices(r.setupCtx, ctrl, discoverOptions{
		MaxHops: r.discoveryHopLimit,
	})
	if err != nil {
		err := fmt.Errorf("service discovery failed: %w", err)
		return err
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

	if TracingEnabled {
		r.setupSpan.AddEvent("Registering Local Service", trace.WithAttributes(
			attribute.String("service", serviceDesc.ServiceName),
		))
	}

	reflectionDesc, err := LoadServiceDesc(serviceDesc)
	if err != nil {
		log.Fatalf("totem: failed to load service descriptor: %v", err)
	}

	r.controller.RegisterServiceHandler(NewDefaultServiceHandler(r.Context(), reflectionDesc,
		newLocalServiceInvoker(impl, serviceDesc, r.logger, r.interceptors.Incoming, r.metrics, 0)))
}

// Serve starts the totem server, which takes control of the stream and begins
// handling incoming and outgoing RPCs.
func (r *Server) Serve() (grpc.ClientConnInterface, <-chan error) {
	r.lock.RLock()
	r.logger.Debug("starting totem server")
	if TracingEnabled {
		r.setupSpan.AddEvent("Starting Server")
	}
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

	info, err := discoverServices(r.setupCtx, r.controller, discoverOptions{
		MaxHops: int32(r.discoveryHopLimit),
	})
	if err != nil {
		r.controller.Kick(fmt.Errorf("service discovery failed: %w", err))
		r.controller.CloseOrRecv()
		ch <- err
		return nil, ch
	}

	r.logger.With(
		zap.Any("methods", info.MethodNames()),
	).Debug("service discovery complete")

	r.setupSpan.AddEvent("Service Discovery Complete", trace.WithAttributes(
		attribute.StringSlice("services", info.ServiceNames()),
	))

	invoker := r.controller.NewInvoker()
	for _, ctrl := range r.splicedControllers {
		for _, svcDesc := range info.Services {
			ctrl.RegisterServiceHandler(NewDefaultServiceHandler(r.Context(), svcDesc, invoker))
		}
		if TracingEnabled {
			r.setupSpan.AddEvent("Syncing Spliced Controller", trace.WithAttributes(
				attribute.String("controller", ctrl.Name),
			))
		}
	}

	r.setupSpan.End(trace.WithTimestamp(time.Now()))

	return &ClientConn{
		controller:  r.controller,
		interceptor: r.interceptors.Outgoing,
		logger:      r.logger.Named("cc"),
		metrics:     r.metrics,
	}, ch
}

// Returns the server's stream context. Only valid after Serve has been called.
func (r *Server) Context() context.Context {
	return r.stream.Context()
}
