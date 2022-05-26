package totem

import (
	"fmt"
	"time"

	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/descriptorpb"
)

var ErrTimeout = fmt.Errorf("timed out")

func WaitErrOrTimeout(errC <-chan error, timeout time.Duration) error {
	select {
	case err := <-errC:
		return err
	case <-time.After(timeout):
		return ErrTimeout
	}
}

func LoadServiceDesc(svc *grpc.ServiceDesc) (*descriptorpb.ServiceDescriptorProto, error) {
	desc, err := grpcreflect.LoadServiceDescriptor(svc)
	if err != nil {
		return nil, err
	}
	dpb := desc.AsServiceDescriptorProto()
	fqn := desc.GetFullyQualifiedName()
	dpb.Name = &fqn
	return dpb, nil
}
