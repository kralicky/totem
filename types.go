package totem

import (
	"sync"

	"golang.org/x/exp/slices"
	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ServerStream interface {
	Stream
	grpc.ServerStream
}

type ClientStream interface {
	Stream
	grpc.ClientStream
}

type Stream interface {
	Send(*RPC) error
	Recv() (*RPC, error)
	Context() context.Context
}

type ServiceHandler struct {
	controllerContext context.Context
	Descriptor        *descriptorpb.ServiceDescriptorProto
	MethodInvokers    map[string]MethodInvoker
}

func (s *ServiceHandler) Done() <-chan struct{} {
	return s.controllerContext.Done()
}

func NewDefaultServiceHandler(
	ctx context.Context,
	descriptor *descriptorpb.ServiceDescriptorProto,
	invoker MethodInvoker,
) *ServiceHandler {
	sh := &ServiceHandler{
		controllerContext: ctx,
		Descriptor:        descriptor,
		MethodInvokers:    make(map[string]MethodInvoker),
	}
	for _, method := range descriptor.Method {
		sh.MethodInvokers[method.GetName()] = invoker
	}
	return sh
}

type ServiceHandlerList struct {
	mu   sync.RWMutex
	data []*ServiceHandler
}

func (s *ServiceHandlerList) Append(sh *ServiceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = append(s.data, sh)

	go func() {
		<-sh.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		if i := slices.Index(s.data, sh); i != -1 {
			s.data = slices.Delete(s.data, i, i+1)
		}
	}()
}

func (s *ServiceHandlerList) Range(fn func(sh *ServiceHandler) bool) {
	s.mu.RLock()
	data := slices.Clone(s.data)
	s.mu.RUnlock()
	for _, sh := range data {
		if !fn(sh) {
			return
		}
	}
}

func (s *ServiceHandlerList) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

func (s *ServiceHandlerList) First() *ServiceHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data) == 0 {
		return nil
	}
	return s.data[0]
}
