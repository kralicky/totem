package totem

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func discoverServices(ctx context.Context, ctrl *StreamController, maxHops int32) (*ServiceInfo, error) {
	reqBytes, _ := proto.Marshal(&DiscoveryRequest{
		Initiator:     ctrl.uuid,
		RemainingHops: maxHops,
	})

	lg := ctrl.logger
	var span trace.Span
	if TracingEnabled {
		ctx, span = Tracer().Start(ctx, "totem.discoverServices")
		defer span.End()
		lg = lg.With(
			zap.String("traceID", span.SpanContext().TraceID().String()),
		)
	}
	lg.Debug("starting service discovery")

	respC := ctrl.Request(ctx, &RPC{
		ServiceName: "totem.ServerReflection",
		MethodName:  "ListServices",
		Content: &RPC_Request{
			Request: reqBytes,
		},
	})

	resp := <-respC
	respMsg := resp.GetResponse()
	stat := respMsg.GetStatus()
	if err := stat.Err(); err != nil {
		lg.With(
			zap.Error(err),
		).Warn("discovery failed")
		return nil, err
	}

	infoMsg := &ServiceInfo{}
	if err := proto.Unmarshal(respMsg.GetResponse(), infoMsg); err != nil {
		lg.Warn("received bad service info message")
		return nil, err
	}

	if TracingEnabled {
		span.AddEvent("Results", trace.WithAttributes(
			attribute.StringSlice("methods", infoMsg.MethodNames()),
		))
	}

	lg.With(
		zap.Any("methods", infoMsg.MethodNames()),
	).Debug("discovery complete")

	return infoMsg, nil
}
