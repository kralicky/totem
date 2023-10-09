package totem

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

type ClientConn struct {
	controller  *StreamController
	interceptor grpc.UnaryClientInterceptor
	logger      *zap.Logger
	metrics     *MetricsExporter
}

var _ grpc.ClientConnInterface = (*ClientConn)(nil)

// Method placeholder to distinguish forwarded raw RPC messages.
const Forward = "(forward)"

func (cc *ClientConn) Invoke(
	ctx context.Context,
	method string,
	req any,
	reply any,
	callOpts ...grpc.CallOption,
) error {
	if cc.interceptor != nil {
		// Important: the interceptor is called with a nil *grpc.ClientConn.
		return cc.interceptor(ctx, method, req, reply, nil, cc.invoke, callOpts...)
	}
	return cc.invoke(ctx, method, req, reply, nil, callOpts...)
}

func (cc *ClientConn) invoke(
	ctx context.Context,
	method string,
	req any,
	reply any,
	_ *grpc.ClientConn,
	callOpts ...grpc.CallOption,
) error {
	var serviceName, methodName string
	if method != Forward {
		var err error
		serviceName, methodName, err = parseQualifiedMethod(method)
		if err != nil {
			return err
		}
	}

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	mdSupplier := metadataSupplier{&md}

	lg := cc.logger.With(
		zap.String("requestType", fmt.Sprintf("%T", req)),
		zap.String("replyType", fmt.Sprintf("%T", reply)),
	)
	var reqMsg []byte
	switch req := req.(type) {
	case *RPC:
		serviceName = req.ServiceName
		methodName = req.MethodName
		lg.With(
			zap.String("method", method),
		).Debug("forwarding rpc")
		reqMsg = req.GetRequest()
		if req.Metadata != nil {
			md = metadata.Join(md, req.Metadata.ToMD())
		}
	case protoadapt.MessageV2:
		lg.With(
			zap.String("method", method),
		).Debug("invoking method")
		var err error
		reqMsg, err = proto.Marshal(req)
		if err != nil {
			return err
		}
	case protoadapt.MessageV1:
		reqv2 := protoadapt.MessageV2Of(req)
		lg.With(
			zap.String("method", method),
		).Debug("invoking method")
		var err error
		reqMsg, err = proto.Marshal(reqv2)
		if err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("[totem] unsupported request type: %T", req))
	}

	var span trace.Span
	if TracingEnabled {
		name, attr := spanInfo(method, peerFromCtx(ctx))
		attr = append(attr, attribute.String("func", "clientConn.Invoke"))
		ctx, span = cc.controller.tracer.Start(ctx, name,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(attr...),
		)
		defer span.End()
		otel.GetTextMapPropagator().Inject(ctx, &mdSupplier)
	}
	cc.metrics.TrackTxBytes(serviceName, methodName, int64(len(reqMsg)))

	rpc := &RPC{
		ServiceName: serviceName,
		MethodName:  methodName,
		Content: &RPC_Request{
			Request: reqMsg,
		},
		Metadata: FromMD(md),
	}

	startTime := time.Now()
	future := cc.controller.Request(ctx, rpc)
	select {
	case rpc := <-future:
		resp := rpc.GetResponse()
		stat := resp.GetStatus()
		if err := stat.Err(); err != nil {
			cc.logger.With(
				zap.Uint64("tag", rpc.Tag),
				zap.String("method", method),
				zap.Error(err),
			).Debug("received reply with error")
			recordErrorStatus(span, stat)
			return err
		}

		cc.logger.With(
			zap.Uint64("tag", rpc.Tag),
			zap.String("method", method),
		).Debug("received reply")
		recordSuccess(span)
		cc.metrics.TrackSvcTxLatency(serviceName, methodName, time.Since(startTime))
		cc.metrics.TrackRxBytes(serviceName, methodName, int64(len(resp.Response)))

		for _, callOpt := range callOpts {
			switch opt := callOpt.(type) {
			case grpc.HeaderCallOption:
				*opt.HeaderAddr = rpc.Metadata.ToMD()
			case grpc.TrailerCallOption:
				*opt.TrailerAddr = rpc.Metadata.ToMD()
			}
		}

		switch reply := reply.(type) {
		case *RPC:
			reply.Content = &RPC_Response{
				Response: resp,
			}
		case protoadapt.MessageV2:
			if err := proto.Unmarshal(resp.GetResponse(), reply); err != nil {
				cc.logger.With(
					zap.Uint64("tag", rpc.Tag),
					zap.String("method", method),
					zap.Error(err),
				).Error("received malformed response message")

				return fmt.Errorf("[totem] malformed response: %w", err)
			}
		case protoadapt.MessageV1:
			replyv2 := protoadapt.MessageV2Of(reply)
			if err := proto.Unmarshal(resp.GetResponse(), replyv2); err != nil {
				cc.logger.With(
					zap.Uint64("tag", rpc.Tag),
					zap.String("method", method),
					zap.Error(err),
				).Error("received malformed response message")

				return fmt.Errorf("[totem] malformed response: %w", err)
			}
		default:
			panic(fmt.Sprintf("[totem] unsupported request type: %T", req))
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cc *ClientConn) NewStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	if desc.ClientStreams {
		panic("[totem] client streaming not implemented")
	}
	serviceName, methodName, err := parseQualifiedMethod(method)
	if err != nil {
		return nil, err
	}
	return newServerStreamClientWrapper(ctx, cc.controller, serviceName, methodName, opts...), nil
}
