package totem

import (
	"fmt"
	"log"
	"reflect"
	"sync"

	"google.golang.org/grpc"
)

type Controller[S stream] struct {
	lock     *sync.Mutex
	types    map[reflect.Type]string // type -> name
	services map[string]*serviceInfo // name -> info

	liveStream S
}

func NewController[S stream](live ...S) *Controller[S] {
	var liveStream S
	if len(live) == 1 {
		liveStream = live[0]
	}
	return &Controller[S]{
		lock:       &sync.Mutex{},
		types:      map[reflect.Type]string{},
		services:   make(map[string]*serviceInfo),
		liveStream: liveStream,
	}
}

type serviceInfo struct {
	serviceImpl interface{}
	methods     map[string]*grpc.MethodDesc
	mdata       interface{}
}

func (r *Controller[S]) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
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

func (r *Controller[S]) register(desc *grpc.ServiceDesc, impl interface{}, st reflect.Type) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if _, ok := r.services[desc.ServiceName]; ok {
		log.Fatalf("grpc: Server.RegisterService found duplicate service registration for %q", desc.ServiceName)
	}
	info := &serviceInfo{
		serviceImpl: impl,
		methods:     make(map[string]*grpc.MethodDesc),
		mdata:       desc.Metadata,
	}
	for i := range desc.Methods {
		d := &desc.Methods[i]
		info.methods[fmt.Sprintf("/%s/%s", desc.ServiceName, d.MethodName)] = d
	}
	r.types[st] = desc.ServiceName
	r.services[desc.ServiceName] = info
}

func (r *Controller[S]) handle(stream S, info *serviceInfo) (grpc.ClientConnInterface, <-chan error) {
	cc := &clientConn{
		handler: newStreamHandler(stream, info),
	}
	ch := make(chan error, 1)
	go func() {
		ch <- cc.handler.Run()
	}()
	return cc, ch
}

func (r *Controller[S]) isLive() bool {
	var zero S
	return r.liveStream != zero
}

func Handle[Srv any, S stream](c *Controller[S], s ...S) (grpc.ClientConnInterface, <-chan error) {
	var stream S
	if len(s) == 1 {
		stream = s[0]
	} else if c.isLive() {
		stream = c.liveStream
	} else {
		panic("controller has no stream")
	}

	var srv Srv
	st := reflect.Indirect(reflect.ValueOf(&srv)).Type()
	c.lock.Lock()
	serviceName, ok := c.types[st]
	if !ok {
		c.lock.Unlock()
		panic(fmt.Sprintf("unknown service %v", st))
	}
	info := c.services[serviceName]
	c.lock.Unlock()
	return c.handle(stream, info)
}
