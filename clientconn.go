package totem

import (
	"errors"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type clientConn struct {
	handler *streamHandler
}

func (cc *clientConn) Invoke(
	ctx context.Context,
	method string,
	args interface{},
	reply interface{},
	opts ...grpc.CallOption,
) error {
	req, err := proto.Marshal(args.(proto.Message))
	if err != nil {
		return err
	}
	future := cc.handler.Request(&RPC{
		Method: method,
		Content: &RPC_Request{
			Request: req,
		},
	})
	select {
	case rpc := <-future:
		resp := rpc.GetResponse()
		if err := resp.GetError(); err != nil {
			return errors.New(string(err))
		}
		if resp == nil {
			return status.Error(codes.Internal, "response was nil")
		}
		return proto.Unmarshal(resp.GetResponse(), reply.(proto.Message))
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cc *clientConn) NewStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	panic("stuck in limbo")
}
