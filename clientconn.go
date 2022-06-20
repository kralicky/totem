package totem

import (
	"fmt"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type clientConn struct {
	handler     *streamHandler
	interceptor grpc.UnaryClientInterceptor
	tracer      trace.Tracer
}

func (cc *clientConn) Invoke(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	callOpts ...grpc.CallOption,
) error {
	reqMsg, err := proto.Marshal(req.(proto.Message))
	if err != nil {
		return err
	}
	rpc := &RPC{
		Method: method,
		Content: &RPC_Request{
			Request: reqMsg,
		},
	}

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	name, attr := spanInfo(method, peerFromCtx(ctx))
	ctx, span := cc.tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attr...),
	)
	defer span.End()
	otelgrpc.Inject(ctx, &md)
	rpc.Metadata = FromMD(md)

	future := cc.handler.Request(rpc)
	select {
	case rpc := <-future:
		resp := rpc.GetResponse()
		stat := resp.GetStatus()
		if err := stat.Err(); err != nil {
			recordErrorStatus(span, stat)
			return err
		}
		recordSuccess(span)

		for _, callOpt := range callOpts {
			switch opt := callOpt.(type) {
			case grpc.HeaderCallOption:
				*opt.HeaderAddr = rpc.Metadata.ToMD()
			case grpc.TrailerCallOption:
				*opt.TrailerAddr = rpc.Metadata.ToMD()
			}
		}
		if err := proto.Unmarshal(resp.GetResponse(), reply.(proto.Message)); err != nil {
			return fmt.Errorf("[totem] malformed response: %w", err)
		}
		return nil
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
	panic("stuck in limbo (nested streams not supported)")
}
