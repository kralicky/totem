package totem

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"

	"go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Server struct {
	lock     *sync.Mutex
	methods  map[string]MethodInvoker
	services map[string]*grpc.ServiceDesc
	stream   Stream
	sctx     context.Context
}

func NewServer(stream Stream) *Server {
	return &Server{
		lock:     &sync.Mutex{},
		methods:  make(map[string]MethodInvoker),
		services: make(map[string]*grpc.ServiceDesc),
		stream:   stream,
	}
}

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
func (r *Server) Splice(stream Stream, descs ...*descriptorpb.ServiceDescriptorProto) {
	if len(descs) == 0 {
		return
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	handler := newStreamHandler(stream, nil)
	for _, desc := range descs {
		for _, method := range desc.Method {
			methodName := fmt.Sprintf("/%s/%s", desc.GetName(), method.GetName())
			r.methods[methodName] = &splicedStreamInvoker{
				handler: handler,
				method:  methodName,
			}
		}
	}
}

func (r *Server) register(desc *grpc.ServiceDesc, impl interface{}) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if _, ok := r.services[desc.ServiceName]; ok {
		log.Fatalf("grpc: Server.RegisterService found duplicate service registration for %q", desc.ServiceName)
	}
	r.services[desc.ServiceName] = desc
	for i := range desc.Methods {
		d := &desc.Methods[i]
		r.methods[fmt.Sprintf("/%s/%s", desc.ServiceName, d.MethodName)] = &localServiceMethod{
			serviceImpl: impl,
			method:      d,
		}
	}
}

// Serve starts the totem server, which takes control of the stream and begins
// handling incoming and outgoing RPCs.
//
// Optionally, if one non-nil channel is passed to this function, the server
// will wait until the channel is closed before starting. This can be used to
// prevent race conditions if you want to interact with the returned ClientConn
// and prevent the server from invoking any message handlers while doing so.
func (r *Server) Serve(condition ...chan struct{}) (grpc.ClientConnInterface, <-chan error) {
	r.lock.Lock()
	cc := &clientConn{
		handler: newStreamHandler(r.stream, r.methods),
		tracer:  Tracer(),
	}
	ch := make(chan error, 1)
	ctx, ca := context.WithCancel(r.stream.Context())
	ctx, span := cc.tracer.Start(ctx, "Serve")
	r.sctx = ctx
	go func() {
		defer ca()
		if len(condition) == 1 && condition[0] != nil {
			_, waitSpan := cc.tracer.Start(ctx, "WaitForCondition")
			<-condition[0]
			waitSpan.End()
		}

		splicedHandlers := map[*streamHandler]struct{}{}
		for _, m := range r.methods {
			if spliced, ok := m.(*splicedStreamInvoker); ok {
				splicedHandlers[spliced.handler] = struct{}{}
			}
		}
		for sh := range splicedHandlers {
			sh := sh
			go func() {
				ch <- fmt.Errorf("[spliced] %w", sh.Run(ctx))
			}()
		}
		runErr := cc.handler.Run(ctx)
		if runErr != nil {
			span.SetStatus(codes.Error, runErr.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		ch <- runErr
		r.lock.Unlock()
		span.End()
	}()
	return cc, ch
}

// Returns the server's stream context. Only valid after Serve has been called.
func (r *Server) Context() context.Context {
	return r.sctx
}
