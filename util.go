package totem

import (
	"fmt"
	"sort"
	"strings"
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

func (i *ServiceInfo) MethodNames() []string {
	var names []string
	for _, svc := range i.GetServices() {
		for _, m := range svc.GetMethod() {
			names = append(names, fmt.Sprintf("/%s/%s", svc.GetName(), m.GetName()))
		}
	}
	sort.Strings(names)
	return names
}

// parses a method name of the form "/service/method"
func parseQualifiedMethod(method string) (string, string, error) {
	parts := strings.Split(method, "/")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid method name: %s", method)
	}
	return parts[1], parts[2], nil
}

func (r *RPC) QualifiedMethodName() string {
	switch r.GetContent().(type) {
	case *RPC_Request:
		return fmt.Sprintf("/%s/%s", r.GetServiceName(), r.GetMethodName())
	case *RPC_Response:
		return fmt.Sprintf("(response)")
	default:
		return fmt.Sprintf("(unknown)")
	}
}
