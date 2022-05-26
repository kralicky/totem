package totem

import (
	"fmt"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type MethodInvoker interface {
	Invoke(ctx context.Context, req []byte) ([]byte, error)
}

type localServiceMethod struct {
	serviceImpl interface{}
	method      *grpc.MethodDesc
}

func (l *localServiceMethod) Invoke(ctx context.Context, req []byte) ([]byte, error) {
	resp, err := l.method.Handler(l.serviceImpl, addTotemToContext(ctx), func(v any) error {
		return proto.Unmarshal(req, v.(proto.Message))
	}, nil)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(resp.(proto.Message))
}

type splicedStreamInvoker struct {
	handler *streamHandler
	method  string
}

func (r *splicedStreamInvoker) Invoke(ctx context.Context, req []byte) ([]byte, error) {
	rc := r.handler.Request(&RPC{
		Method: r.method,
		Content: &RPC_Request{
			Request: req,
		},
	})
	select {
	case rpc := <-rc:
		resp := rpc.GetResponse()
		if resp.Error != nil {
			return nil, fmt.Errorf("[spliced invoker] %s", string(resp.Error))
		}
		return resp.GetResponse(), nil
	}
}
