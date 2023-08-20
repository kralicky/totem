package totem

import (
	"context"
	"strings"
	"sync"

	"slices"

	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type (
	TopologyFlags int
)

const (
	TopologyLocal TopologyFlags = 1 << iota
	TopologySelf
	TopologySpliced
)

func (tf TopologyFlags) DisplayName() string {
	names := []string{"local", "self", "spliced"}
	var flags []string
	for i, name := range names {
		if tf&(1<<i) != 0 {
			flags = append(flags, name)
		}
	}
	if tf&(TopologySelf|TopologyLocal) == 0 {
		flags = append(flags, "remote")
	}
	return strings.Join(flags, "|")
}

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
	MethodQOS         map[string]*QOS
	TopologyFlags     TopologyFlags
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
		MethodQOS:         make(map[string]*QOS),
		TopologyFlags:     invoker.TopologyFlags(),
	}
	for _, method := range descriptor.Method {
		if proto.HasExtension(method.GetOptions(), E_Qos) {
			qos := proto.GetExtension(method.GetOptions(), E_Qos).(*QOS)
			if qos.ReplicationStrategy == ReplicationStrategy_Broadcast && method.GetOutputType() != ".google.protobuf.Empty" {
				// todo: temporary restriction
				panic("methods with Broadcast ReplicationStrategy must have a response type of google.protobuf.Empty")
			}
			sh.MethodQOS[method.GetName()] = qos
		}
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

	for _, existing := range s.data {
		if !proto.Equal(existing.Descriptor, sh.Descriptor) {
			panic("entries in ServiceHandlerLists must have the same service descriptors")
		}
	}

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

func (s *ServiceHandlerList) Range(fn func(sh *ServiceHandler) bool) bool {
	s.mu.RLock()
	data := slices.Clone(s.data)
	s.mu.RUnlock()
	for _, sh := range data {
		if !fn(sh) {
			return false
		}
	}
	return true
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
