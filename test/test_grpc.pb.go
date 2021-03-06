// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - ragù               v0.2.3
// source: test/test.proto

package test

import (
	context "context"
	totem "github.com/kralicky/totem"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// TestClient is the client API for Test service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TestClient interface {
	TestStream(ctx context.Context, opts ...grpc.CallOption) (Test_TestStreamClient, error)
}

type testClient struct {
	cc grpc.ClientConnInterface
}

func NewTestClient(cc grpc.ClientConnInterface) TestClient {
	return &testClient{cc}
}

func (c *testClient) TestStream(ctx context.Context, opts ...grpc.CallOption) (Test_TestStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &Test_ServiceDesc.Streams[0], "/test.Test/TestStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &testTestStreamClient{stream}
	return x, nil
}

type Test_TestStreamClient interface {
	Send(*totem.RPC) error
	Recv() (*totem.RPC, error)
	grpc.ClientStream
}

type testTestStreamClient struct {
	grpc.ClientStream
}

func (x *testTestStreamClient) Send(m *totem.RPC) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testTestStreamClient) Recv() (*totem.RPC, error) {
	m := new(totem.RPC)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// TestServer is the server API for Test service.
// All implementations must embed UnimplementedTestServer
// for forward compatibility
type TestServer interface {
	TestStream(Test_TestStreamServer) error
	mustEmbedUnimplementedTestServer()
}

// UnimplementedTestServer must be embedded to have forward compatible implementations.
type UnimplementedTestServer struct {
}

func (UnimplementedTestServer) TestStream(Test_TestStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method TestStream not implemented")
}
func (UnimplementedTestServer) mustEmbedUnimplementedTestServer() {}

// UnsafeTestServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TestServer will
// result in compilation errors.
type UnsafeTestServer interface {
	mustEmbedUnimplementedTestServer()
}

func RegisterTestServer(s grpc.ServiceRegistrar, srv TestServer) {
	s.RegisterService(&Test_ServiceDesc, srv)
}

func _Test_TestStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TestServer).TestStream(&testTestStreamServer{stream})
}

type Test_TestStreamServer interface {
	Send(*totem.RPC) error
	Recv() (*totem.RPC, error)
	grpc.ServerStream
}

type testTestStreamServer struct {
	grpc.ServerStream
}

func (x *testTestStreamServer) Send(m *totem.RPC) error {
	return x.ServerStream.SendMsg(m)
}

func (x *testTestStreamServer) Recv() (*totem.RPC, error) {
	m := new(totem.RPC)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Test_ServiceDesc is the grpc.ServiceDesc for Test service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Test_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.Test",
	HandlerType: (*TestServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "TestStream",
			Handler:       _Test_TestStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "test/test.proto",
}

// IncrementClient is the client API for Increment service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type IncrementClient interface {
	Inc(ctx context.Context, in *Number, opts ...grpc.CallOption) (*Number, error)
}

type incrementClient struct {
	cc grpc.ClientConnInterface
}

func NewIncrementClient(cc grpc.ClientConnInterface) IncrementClient {
	return &incrementClient{cc}
}

func (c *incrementClient) Inc(ctx context.Context, in *Number, opts ...grpc.CallOption) (*Number, error) {
	out := new(Number)
	err := c.cc.Invoke(ctx, "/test.Increment/Inc", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// IncrementServer is the server API for Increment service.
// All implementations must embed UnimplementedIncrementServer
// for forward compatibility
type IncrementServer interface {
	Inc(context.Context, *Number) (*Number, error)
	mustEmbedUnimplementedIncrementServer()
}

// UnimplementedIncrementServer must be embedded to have forward compatible implementations.
type UnimplementedIncrementServer struct {
}

func (UnimplementedIncrementServer) Inc(context.Context, *Number) (*Number, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Inc not implemented")
}
func (UnimplementedIncrementServer) mustEmbedUnimplementedIncrementServer() {}

// UnsafeIncrementServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to IncrementServer will
// result in compilation errors.
type UnsafeIncrementServer interface {
	mustEmbedUnimplementedIncrementServer()
}

func RegisterIncrementServer(s grpc.ServiceRegistrar, srv IncrementServer) {
	s.RegisterService(&Increment_ServiceDesc, srv)
}

func _Increment_Inc_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Number)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IncrementServer).Inc(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/test.Increment/Inc",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IncrementServer).Inc(ctx, req.(*Number))
	}
	return interceptor(ctx, in, info, handler)
}

// Increment_ServiceDesc is the grpc.ServiceDesc for Increment service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Increment_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.Increment",
	HandlerType: (*IncrementServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Inc",
			Handler:    _Increment_Inc_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "test/test.proto",
}

// DecrementClient is the client API for Decrement service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type DecrementClient interface {
	Dec(ctx context.Context, in *Number, opts ...grpc.CallOption) (*Number, error)
}

type decrementClient struct {
	cc grpc.ClientConnInterface
}

func NewDecrementClient(cc grpc.ClientConnInterface) DecrementClient {
	return &decrementClient{cc}
}

func (c *decrementClient) Dec(ctx context.Context, in *Number, opts ...grpc.CallOption) (*Number, error) {
	out := new(Number)
	err := c.cc.Invoke(ctx, "/test.Decrement/Dec", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DecrementServer is the server API for Decrement service.
// All implementations must embed UnimplementedDecrementServer
// for forward compatibility
type DecrementServer interface {
	Dec(context.Context, *Number) (*Number, error)
	mustEmbedUnimplementedDecrementServer()
}

// UnimplementedDecrementServer must be embedded to have forward compatible implementations.
type UnimplementedDecrementServer struct {
}

func (UnimplementedDecrementServer) Dec(context.Context, *Number) (*Number, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Dec not implemented")
}
func (UnimplementedDecrementServer) mustEmbedUnimplementedDecrementServer() {}

// UnsafeDecrementServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to DecrementServer will
// result in compilation errors.
type UnsafeDecrementServer interface {
	mustEmbedUnimplementedDecrementServer()
}

func RegisterDecrementServer(s grpc.ServiceRegistrar, srv DecrementServer) {
	s.RegisterService(&Decrement_ServiceDesc, srv)
}

func _Decrement_Dec_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Number)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DecrementServer).Dec(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/test.Decrement/Dec",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DecrementServer).Dec(ctx, req.(*Number))
	}
	return interceptor(ctx, in, info, handler)
}

// Decrement_ServiceDesc is the grpc.ServiceDesc for Decrement service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Decrement_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.Decrement",
	HandlerType: (*DecrementServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Dec",
			Handler:    _Decrement_Dec_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "test/test.proto",
}

// HashClient is the client API for Hash service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type HashClient interface {
	Hash(ctx context.Context, in *String, opts ...grpc.CallOption) (*String, error)
}

type hashClient struct {
	cc grpc.ClientConnInterface
}

func NewHashClient(cc grpc.ClientConnInterface) HashClient {
	return &hashClient{cc}
}

func (c *hashClient) Hash(ctx context.Context, in *String, opts ...grpc.CallOption) (*String, error) {
	out := new(String)
	err := c.cc.Invoke(ctx, "/test.Hash/Hash", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// HashServer is the server API for Hash service.
// All implementations must embed UnimplementedHashServer
// for forward compatibility
type HashServer interface {
	Hash(context.Context, *String) (*String, error)
	mustEmbedUnimplementedHashServer()
}

// UnimplementedHashServer must be embedded to have forward compatible implementations.
type UnimplementedHashServer struct {
}

func (UnimplementedHashServer) Hash(context.Context, *String) (*String, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Hash not implemented")
}
func (UnimplementedHashServer) mustEmbedUnimplementedHashServer() {}

// UnsafeHashServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to HashServer will
// result in compilation errors.
type UnsafeHashServer interface {
	mustEmbedUnimplementedHashServer()
}

func RegisterHashServer(s grpc.ServiceRegistrar, srv HashServer) {
	s.RegisterService(&Hash_ServiceDesc, srv)
}

func _Hash_Hash_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(String)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HashServer).Hash(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/test.Hash/Hash",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HashServer).Hash(ctx, req.(*String))
	}
	return interceptor(ctx, in, info, handler)
}

// Hash_ServiceDesc is the grpc.ServiceDesc for Hash service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Hash_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.Hash",
	HandlerType: (*HashServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Hash",
			Handler:    _Hash_Hash_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "test/test.proto",
}

// AddSubClient is the client API for AddSub service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AddSubClient interface {
	Add(ctx context.Context, in *Operands, opts ...grpc.CallOption) (*Number, error)
	Sub(ctx context.Context, in *Operands, opts ...grpc.CallOption) (*Number, error)
}

type addSubClient struct {
	cc grpc.ClientConnInterface
}

func NewAddSubClient(cc grpc.ClientConnInterface) AddSubClient {
	return &addSubClient{cc}
}

func (c *addSubClient) Add(ctx context.Context, in *Operands, opts ...grpc.CallOption) (*Number, error) {
	out := new(Number)
	err := c.cc.Invoke(ctx, "/test.AddSub/Add", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *addSubClient) Sub(ctx context.Context, in *Operands, opts ...grpc.CallOption) (*Number, error) {
	out := new(Number)
	err := c.cc.Invoke(ctx, "/test.AddSub/Sub", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AddSubServer is the server API for AddSub service.
// All implementations must embed UnimplementedAddSubServer
// for forward compatibility
type AddSubServer interface {
	Add(context.Context, *Operands) (*Number, error)
	Sub(context.Context, *Operands) (*Number, error)
	mustEmbedUnimplementedAddSubServer()
}

// UnimplementedAddSubServer must be embedded to have forward compatible implementations.
type UnimplementedAddSubServer struct {
}

func (UnimplementedAddSubServer) Add(context.Context, *Operands) (*Number, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Add not implemented")
}
func (UnimplementedAddSubServer) Sub(context.Context, *Operands) (*Number, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Sub not implemented")
}
func (UnimplementedAddSubServer) mustEmbedUnimplementedAddSubServer() {}

// UnsafeAddSubServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AddSubServer will
// result in compilation errors.
type UnsafeAddSubServer interface {
	mustEmbedUnimplementedAddSubServer()
}

func RegisterAddSubServer(s grpc.ServiceRegistrar, srv AddSubServer) {
	s.RegisterService(&AddSub_ServiceDesc, srv)
}

func _AddSub_Add_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Operands)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AddSubServer).Add(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/test.AddSub/Add",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AddSubServer).Add(ctx, req.(*Operands))
	}
	return interceptor(ctx, in, info, handler)
}

func _AddSub_Sub_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Operands)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AddSubServer).Sub(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/test.AddSub/Sub",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AddSubServer).Sub(ctx, req.(*Operands))
	}
	return interceptor(ctx, in, info, handler)
}

// AddSub_ServiceDesc is the grpc.ServiceDesc for AddSub service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var AddSub_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.AddSub",
	HandlerType: (*AddSubServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Add",
			Handler:    _AddSub_Add_Handler,
		},
		{
			MethodName: "Sub",
			Handler:    _AddSub_Sub_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "test/test.proto",
}

// ErrorClient is the client API for Error service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ErrorClient interface {
	Error(ctx context.Context, in *ErrorRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type errorClient struct {
	cc grpc.ClientConnInterface
}

func NewErrorClient(cc grpc.ClientConnInterface) ErrorClient {
	return &errorClient{cc}
}

func (c *errorClient) Error(ctx context.Context, in *ErrorRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/test.Error/Error", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ErrorServer is the server API for Error service.
// All implementations must embed UnimplementedErrorServer
// for forward compatibility
type ErrorServer interface {
	Error(context.Context, *ErrorRequest) (*emptypb.Empty, error)
	mustEmbedUnimplementedErrorServer()
}

// UnimplementedErrorServer must be embedded to have forward compatible implementations.
type UnimplementedErrorServer struct {
}

func (UnimplementedErrorServer) Error(context.Context, *ErrorRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Error not implemented")
}
func (UnimplementedErrorServer) mustEmbedUnimplementedErrorServer() {}

// UnsafeErrorServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ErrorServer will
// result in compilation errors.
type UnsafeErrorServer interface {
	mustEmbedUnimplementedErrorServer()
}

func RegisterErrorServer(s grpc.ServiceRegistrar, srv ErrorServer) {
	s.RegisterService(&Error_ServiceDesc, srv)
}

func _Error_Error_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ErrorRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ErrorServer).Error(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/test.Error/Error",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ErrorServer).Error(ctx, req.(*ErrorRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Error_ServiceDesc is the grpc.ServiceDesc for Error service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Error_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.Error",
	HandlerType: (*ErrorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Error",
			Handler:    _Error_Error_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "test/test.proto",
}
