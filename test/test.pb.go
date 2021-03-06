// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	ragù          v0.2.3
// source: test/test.proto

package test

import (
	totem "github.com/kralicky/totem"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ErrorRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ReturnError bool `protobuf:"varint,1,opt,name=ReturnError,proto3" json:"ReturnError,omitempty"`
}

func (x *ErrorRequest) Reset() {
	*x = ErrorRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_test_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ErrorRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ErrorRequest) ProtoMessage() {}

func (x *ErrorRequest) ProtoReflect() protoreflect.Message {
	mi := &file_test_test_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ErrorRequest.ProtoReflect.Descriptor instead.
func (*ErrorRequest) Descriptor() ([]byte, []int) {
	return file_test_test_proto_rawDescGZIP(), []int{0}
}

func (x *ErrorRequest) GetReturnError() bool {
	if x != nil {
		return x.ReturnError
	}
	return false
}

type Number struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Value int64 `protobuf:"varint,1,opt,name=Value,proto3" json:"Value,omitempty"`
}

func (x *Number) Reset() {
	*x = Number{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_test_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Number) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Number) ProtoMessage() {}

func (x *Number) ProtoReflect() protoreflect.Message {
	mi := &file_test_test_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Number.ProtoReflect.Descriptor instead.
func (*Number) Descriptor() ([]byte, []int) {
	return file_test_test_proto_rawDescGZIP(), []int{1}
}

func (x *Number) GetValue() int64 {
	if x != nil {
		return x.Value
	}
	return 0
}

type String struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Str string `protobuf:"bytes,1,opt,name=Str,proto3" json:"Str,omitempty"`
}

func (x *String) Reset() {
	*x = String{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_test_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *String) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*String) ProtoMessage() {}

func (x *String) ProtoReflect() protoreflect.Message {
	mi := &file_test_test_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use String.ProtoReflect.Descriptor instead.
func (*String) Descriptor() ([]byte, []int) {
	return file_test_test_proto_rawDescGZIP(), []int{2}
}

func (x *String) GetStr() string {
	if x != nil {
		return x.Str
	}
	return ""
}

type Operands struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	A int32 `protobuf:"varint,1,opt,name=A,proto3" json:"A,omitempty"`
	B int32 `protobuf:"varint,2,opt,name=B,proto3" json:"B,omitempty"`
}

func (x *Operands) Reset() {
	*x = Operands{}
	if protoimpl.UnsafeEnabled {
		mi := &file_test_test_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Operands) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Operands) ProtoMessage() {}

func (x *Operands) ProtoReflect() protoreflect.Message {
	mi := &file_test_test_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Operands.ProtoReflect.Descriptor instead.
func (*Operands) Descriptor() ([]byte, []int) {
	return file_test_test_proto_rawDescGZIP(), []int{3}
}

func (x *Operands) GetA() int32 {
	if x != nil {
		return x.A
	}
	return 0
}

func (x *Operands) GetB() int32 {
	if x != nil {
		return x.B
	}
	return 0
}

var File_test_test_proto protoreflect.FileDescriptor

var file_test_test_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x74, 0x65, 0x73, 0x74, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x04, 0x74, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x0b, 0x74, 0x6f, 0x74, 0x65, 0x6d, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x27, 0x0a, 0x0c, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x15, 0x0a, 0x0b, 0x52, 0x65, 0x74, 0x75, 0x72, 0x6e, 0x45, 0x72, 0x72, 0x6f, 0x72,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x42, 0x00, 0x3a, 0x00, 0x22, 0x1b, 0x0a, 0x06, 0x4e, 0x75,
	0x6d, 0x62, 0x65, 0x72, 0x12, 0x0f, 0x0a, 0x05, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x03, 0x42, 0x00, 0x3a, 0x00, 0x22, 0x19, 0x0a, 0x06, 0x53, 0x74, 0x72, 0x69, 0x6e,
	0x67, 0x12, 0x0d, 0x0a, 0x03, 0x53, 0x74, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x00,
	0x3a, 0x00, 0x22, 0x26, 0x0a, 0x08, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x6e, 0x64, 0x73, 0x12, 0x0b,
	0x0a, 0x01, 0x41, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x42, 0x00, 0x12, 0x0b, 0x0a, 0x01, 0x42,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x42, 0x00, 0x3a, 0x00, 0x32, 0x34, 0x0a, 0x04, 0x54, 0x65,
	0x73, 0x74, 0x12, 0x2a, 0x0a, 0x0a, 0x54, 0x65, 0x73, 0x74, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d,
	0x12, 0x0a, 0x2e, 0x74, 0x6f, 0x74, 0x65, 0x6d, 0x2e, 0x52, 0x50, 0x43, 0x1a, 0x0a, 0x2e, 0x74,
	0x6f, 0x74, 0x65, 0x6d, 0x2e, 0x52, 0x50, 0x43, 0x22, 0x00, 0x28, 0x01, 0x30, 0x01, 0x1a, 0x00,
	0x32, 0x36, 0x0a, 0x09, 0x49, 0x6e, 0x63, 0x72, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x27, 0x0a,
	0x03, 0x49, 0x6e, 0x63, 0x12, 0x0c, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x4e, 0x75, 0x6d, 0x62,
	0x65, 0x72, 0x1a, 0x0c, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72,
	0x22, 0x00, 0x28, 0x00, 0x30, 0x00, 0x1a, 0x00, 0x32, 0x36, 0x0a, 0x09, 0x44, 0x65, 0x63, 0x72,
	0x65, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x27, 0x0a, 0x03, 0x44, 0x65, 0x63, 0x12, 0x0c, 0x2e, 0x74,
	0x65, 0x73, 0x74, 0x2e, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x1a, 0x0c, 0x2e, 0x74, 0x65, 0x73,
	0x74, 0x2e, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x22, 0x00, 0x28, 0x00, 0x30, 0x00, 0x1a, 0x00,
	0x32, 0x32, 0x0a, 0x04, 0x48, 0x61, 0x73, 0x68, 0x12, 0x28, 0x0a, 0x04, 0x48, 0x61, 0x73, 0x68,
	0x12, 0x0c, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x1a, 0x0c,
	0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x00, 0x28, 0x00,
	0x30, 0x00, 0x1a, 0x00, 0x32, 0x60, 0x0a, 0x06, 0x41, 0x64, 0x64, 0x53, 0x75, 0x62, 0x12, 0x29,
	0x0a, 0x03, 0x41, 0x64, 0x64, 0x12, 0x0e, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x4f, 0x70, 0x65,
	0x72, 0x61, 0x6e, 0x64, 0x73, 0x1a, 0x0c, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x4e, 0x75, 0x6d,
	0x62, 0x65, 0x72, 0x22, 0x00, 0x28, 0x00, 0x30, 0x00, 0x12, 0x29, 0x0a, 0x03, 0x53, 0x75, 0x62,
	0x12, 0x0e, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x6e, 0x64, 0x73,
	0x1a, 0x0c, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x22, 0x00,
	0x28, 0x00, 0x30, 0x00, 0x1a, 0x00, 0x32, 0x44, 0x0a, 0x05, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x12,
	0x39, 0x0a, 0x05, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x12, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e,
	0x45, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45,
	0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x28, 0x00, 0x30, 0x00, 0x1a, 0x00, 0x42, 0x20, 0x5a, 0x1e,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6b, 0x72, 0x61, 0x6c, 0x69,
	0x63, 0x6b, 0x79, 0x2f, 0x74, 0x6f, 0x74, 0x65, 0x6d, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_test_test_proto_rawDescOnce sync.Once
	file_test_test_proto_rawDescData = file_test_test_proto_rawDesc
)

func file_test_test_proto_rawDescGZIP() []byte {
	file_test_test_proto_rawDescOnce.Do(func() {
		file_test_test_proto_rawDescData = protoimpl.X.CompressGZIP(file_test_test_proto_rawDescData)
	})
	return file_test_test_proto_rawDescData
}

var file_test_test_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_test_test_proto_goTypes = []interface{}{
	(*ErrorRequest)(nil),  // 0: test.ErrorRequest
	(*Number)(nil),        // 1: test.Number
	(*String)(nil),        // 2: test.String
	(*Operands)(nil),      // 3: test.Operands
	(*totem.RPC)(nil),     // 4: totem.RPC
	(*emptypb.Empty)(nil), // 5: google.protobuf.Empty
}
var file_test_test_proto_depIdxs = []int32{
	4, // 0: test.Test.TestStream:input_type -> totem.RPC
	1, // 1: test.Increment.Inc:input_type -> test.Number
	1, // 2: test.Decrement.Dec:input_type -> test.Number
	2, // 3: test.Hash.Hash:input_type -> test.String
	3, // 4: test.AddSub.Add:input_type -> test.Operands
	3, // 5: test.AddSub.Sub:input_type -> test.Operands
	0, // 6: test.Error.Error:input_type -> test.ErrorRequest
	4, // 7: test.Test.TestStream:output_type -> totem.RPC
	1, // 8: test.Increment.Inc:output_type -> test.Number
	1, // 9: test.Decrement.Dec:output_type -> test.Number
	2, // 10: test.Hash.Hash:output_type -> test.String
	1, // 11: test.AddSub.Add:output_type -> test.Number
	1, // 12: test.AddSub.Sub:output_type -> test.Number
	5, // 13: test.Error.Error:output_type -> google.protobuf.Empty
	7, // [7:14] is the sub-list for method output_type
	0, // [0:7] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_test_test_proto_init() }
func file_test_test_proto_init() {
	if File_test_test_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_test_test_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ErrorRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_test_test_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Number); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_test_test_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*String); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_test_test_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Operands); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_test_test_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   6,
		},
		GoTypes:           file_test_test_proto_goTypes,
		DependencyIndexes: file_test_test_proto_depIdxs,
		MessageInfos:      file_test_test_proto_msgTypes,
	}.Build()
	File_test_test_proto = out.File
	file_test_test_proto_rawDesc = nil
	file_test_test_proto_goTypes = nil
	file_test_test_proto_depIdxs = nil
}
