package totem

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type MethodInvoker interface {
	Invoke(ctx context.Context, rpc *RPC) ([]byte, error)
}

type localServiceMethod struct {
	serviceImpl interface{}
	method      *grpc.MethodDesc
}

func (l *localServiceMethod) Invoke(ctx context.Context, req *RPC) ([]byte, error) {
	ctx, span := Tracer().Start(ctx, "LocalService.Invoke")
	defer span.End()

	ctx = metadata.NewIncomingContext(ctx, req.Metadata.ToMD())
	resp, err := l.method.Handler(l.serviceImpl, addTotemToContext(ctx), func(v any) error {
		return proto.Unmarshal(req.GetRequest(), v.(proto.Message))
	}, nil)
	if err != nil {
		recordError(span, err)
		return nil, err
	}
	recordSuccess(span)
	return proto.Marshal(resp.(proto.Message))
}

type splicedStreamInvoker struct {
	handler *streamHandler
	method  string
}

func (r *splicedStreamInvoker) Invoke(ctx context.Context, req *RPC) ([]byte, error) {
	ctx, span := Tracer().Start(ctx, "SplicedStream.Invoke",
		trace.WithAttributes(
			attribute.String("method", r.method),
			attribute.Int("tag", int(req.Tag)),
		),
	)
	defer span.End()
	rc := r.handler.Request(req)
	select {
	case rpc := <-rc:
		resp := rpc.GetResponse()
		stat := resp.GetStatus()
		if err := stat.Err(); err != nil {
			recordErrorStatus(span, stat)
			return nil, err
		}
		recordSuccess(span)

		return resp.GetResponse(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
