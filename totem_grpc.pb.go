// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - ragù               v0.2.3
// source: totem.proto

package totem

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ServerReflectionClient is the client API for ServerReflection service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ServerReflectionClient interface {
	ListServices(ctx context.Context, in *DiscoveryRequest, opts ...grpc.CallOption) (*ServiceInfo, error)
}

type serverReflectionClient struct {
	cc grpc.ClientConnInterface
}

func NewServerReflectionClient(cc grpc.ClientConnInterface) ServerReflectionClient {
	return &serverReflectionClient{cc}
}

func (c *serverReflectionClient) ListServices(ctx context.Context, in *DiscoveryRequest, opts ...grpc.CallOption) (*ServiceInfo, error) {
	out := new(ServiceInfo)
	err := c.cc.Invoke(ctx, "/totem.ServerReflection/ListServices", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ServerReflectionServer is the server API for ServerReflection service.
// All implementations must embed UnimplementedServerReflectionServer
// for forward compatibility
type ServerReflectionServer interface {
	ListServices(context.Context, *DiscoveryRequest) (*ServiceInfo, error)
	mustEmbedUnimplementedServerReflectionServer()
}

// UnimplementedServerReflectionServer must be embedded to have forward compatible implementations.
type UnimplementedServerReflectionServer struct {
}

func (UnimplementedServerReflectionServer) ListServices(context.Context, *DiscoveryRequest) (*ServiceInfo, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListServices not implemented")
}
func (UnimplementedServerReflectionServer) mustEmbedUnimplementedServerReflectionServer() {}

// UnsafeServerReflectionServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ServerReflectionServer will
// result in compilation errors.
type UnsafeServerReflectionServer interface {
	mustEmbedUnimplementedServerReflectionServer()
}

func RegisterServerReflectionServer(s grpc.ServiceRegistrar, srv ServerReflectionServer) {
	s.RegisterService(&ServerReflection_ServiceDesc, srv)
}

func _ServerReflection_ListServices_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DiscoveryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ServerReflectionServer).ListServices(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/totem.ServerReflection/ListServices",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ServerReflectionServer).ListServices(ctx, req.(*DiscoveryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ServerReflection_ServiceDesc is the grpc.ServiceDesc for ServerReflection service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ServerReflection_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "totem.ServerReflection",
	HandlerType: (*ServerReflectionServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListServices",
			Handler:    _ServerReflection_ListServices_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "totem.proto",
}
