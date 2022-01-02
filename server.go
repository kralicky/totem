package totem

import (
	"fmt"
	"log"
	"reflect"
	"sync"

	"google.golang.org/grpc"
)

type serviceMethod struct {
	serviceImpl interface{}
	method      *grpc.MethodDesc
}

type Server struct {
	lock     *sync.Mutex
	methods  map[string]serviceMethod
	services map[string]*grpc.ServiceDesc
	stream   Stream
}

func NewServer(stream Stream) *Server {
	return &Server{
		lock:     &sync.Mutex{},
		methods:  make(map[string]serviceMethod),
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
		r.register(desc, impl, ht)
	} else {
		log.Fatalf("grpc: Server.RegisterService found nil service implementation")
	}
}

func (r *Server) register(desc *grpc.ServiceDesc, impl interface{}, st reflect.Type) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if _, ok := r.services[desc.ServiceName]; ok {
		log.Fatalf("grpc: Server.RegisterService found duplicate service registration for %q", desc.ServiceName)
	}
	r.services[desc.ServiceName] = desc
	for i := range desc.Methods {
		d := &desc.Methods[i]
		r.methods[fmt.Sprintf("/%s/%s", desc.ServiceName, d.MethodName)] = serviceMethod{
			serviceImpl: impl,
			method:      d,
		}
	}
}

func (r *Server) Serve() (grpc.ClientConnInterface, <-chan error) {
	// permanently lock this mutex to prevent modifications after serving
	r.lock.Lock()
	cc := &clientConn{
		handler: newStreamHandler(r.stream, r.methods),
	}
	ch := make(chan error, 1)
	go func() {
		ch <- cc.handler.Run()
	}()
	return cc, ch
}
