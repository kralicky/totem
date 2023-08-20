package totem

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const TracerName = "totem"

func TracerProvider(opts ...resource.Option) (_tp trace.TracerProvider) {
	defer func() {
		if _tp == nil {
			_tp = otel.GetTracerProvider()
		}
	}()

	if !TracingEnabled {
		return nil
	}

	resourceOpts := []resource.Option{
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	}

	var exporter tracesdk.SpanExporter

	switch os.Getenv("OTEL_TRACES_EXPORTER") {
	case "otlp":
		var err error
		exporter, err = otlptracegrpc.New(context.Background())
		if err != nil {
			return nil
		}
	default:
		return nil
	}

	res, err := resource.New(context.Background(), append(resourceOpts, opts...)...)
	if err != nil {
		return nil
	}
	return tracesdk.NewTracerProvider(tracesdk.WithResource(res), tracesdk.WithBatcher(exporter))
}

// Controls whether or not tracing is enabled. Must only be set once at
// startup. Defaults to false.
var TracingEnabled = false

func init() {
	if v, err := strconv.ParseBool(os.Getenv("TOTEM_TRACING_ENABLED")); err == nil {
		TracingEnabled = v
	}
}

// internal helper methods from otelgrpc below
// -------------------------------------------

// spanInfo returns a span name and all appropriate attributes from the gRPC
// method and peer address.
func spanInfo(fullMethod, peerAddress string) (string, []attribute.KeyValue) {
	attrs := []attribute.KeyValue{otelgrpc.RPCSystemGRPC}
	name, mAttrs := parseFullMethod(fullMethod)
	attrs = append(attrs, mAttrs...)
	attrs = append(attrs, peerAttr(peerAddress)...)
	return name, attrs
}

// peerAttr returns attributes about the peer address.
func peerAttr(addr string) []attribute.KeyValue {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return []attribute.KeyValue(nil)
	}

	if host == "" {
		host = "127.0.0.1"
	}

	return []attribute.KeyValue{
		semconv.NetPeerIPKey.String(host),
		semconv.NetPeerPortKey.String(port),
	}
}

// peerFromCtx returns a peer address from a context, if one exists.
func peerFromCtx(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}
	return p.Addr.String()
}

// ParseFullMethod returns a span name following the OpenTelemetry semantic
// conventions as well as all applicable span attribute.KeyValue attributes based
// on a gRPC's FullMethod.
func parseFullMethod(fullMethod string) (string, []attribute.KeyValue) {
	name := strings.TrimLeft(fullMethod, "/")
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		// Invalid format, does not follow `/package.service/method`.
		return name, []attribute.KeyValue(nil)
	}

	var attrs []attribute.KeyValue
	if service := parts[0]; service != "" {
		attrs = append(attrs, semconv.RPCServiceKey.String(service))
	}
	if method := parts[1]; method != "" {
		attrs = append(attrs, semconv.RPCMethodKey.String(method))
	}
	return name, attrs
}

// statusCodeAttr returns status code attribute based on given gRPC code
func statusCodeAttr(c codes.Code) attribute.KeyValue {
	return otelgrpc.GRPCStatusCodeKey.Int64(int64(c))
}

func recordErrorStatus(span trace.Span, stat *status.Status) {
	if !TracingEnabled {
		return
	}
	span.SetAttributes(statusCodeAttr(stat.Code()))
	span.SetStatus(otelcodes.Error, stat.Message())
	span.RecordError(stat.Err())
}

func recordError(span trace.Span, err error) {
	if !TracingEnabled {
		return
	}
	span.SetAttributes(statusCodeAttr(status.Code(err)))
	span.SetStatus(otelcodes.Error, err.Error())
	span.RecordError(err)
}

func recordSuccess(span trace.Span) {
	if !TracingEnabled {
		return
	}
	span.SetAttributes(statusCodeAttr(codes.OK))
}

// unexported code copied from otelgrpc/metadata_supplier.go

type metadataSupplier struct {
	metadata *metadata.MD
}

// assert that metadataSupplier implements the TextMapCarrier interface.
var _ propagation.TextMapCarrier = &metadataSupplier{}

func (s *metadataSupplier) Get(key string) string {
	values := s.metadata.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (s *metadataSupplier) Set(key string, value string) {
	s.metadata.Set(key, value)
}

func (s *metadataSupplier) Keys() []string {
	out := make([]string, 0, len(*s.metadata))
	for key := range *s.metadata {
		out = append(out, key)
	}
	return out
}
