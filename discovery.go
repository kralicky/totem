package totem

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type discoverOptions struct {
	MaxHops int32
}

func discoverServices(ctx context.Context, ctrl *StreamController, opts discoverOptions) (*ServiceInfo, error) {
	reqBytes, _ := proto.Marshal(&DiscoveryRequest{
		Initiator:     ctrl.uuid,
		Visited:       []string{ctrl.uuid},
		RemainingHops: opts.MaxHops,
	})

	lg := ctrl.Logger
	var span trace.Span
	if TracingEnabled {
		ctx, span = ctrl.tracer.Start(ctx, "Discovery",
			trace.WithLinks(trace.LinkFromContext(ctrl.stream.Context())),
			trace.WithAttributes(
				attribute.String("initiator", ctrl.uuid),
				attribute.String("name", ctrl.Name),
				attribute.Int("maxHops", int(opts.MaxHops)),
			),
		)
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
		span.AddEvent("Completed", trace.WithAttributes(
			attribute.StringSlice("services", infoMsg.ServiceNames()),
		), trace.WithTimestamp(time.Now()))
	}

	lg.With(
		zap.Any("services", infoMsg.ServiceNames()),
	).Debug("discovery complete")

	return infoMsg, nil
}
