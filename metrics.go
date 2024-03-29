package totem

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type MetricsExporter struct {
	rxBytesCollector metric.Int64Counter
	txBytesCollector metric.Int64Counter

	rxRpcCollector metric.Int64Counter
	txRpcCollector metric.Int64Counter

	svcRxLatencyCollector metric.Int64Histogram
	svcTxLatencyCollector metric.Int64Histogram

	staticAttrs []attribute.KeyValue
}

func NewMetricsExporter(provider metric.MeterProvider, staticAttrs ...attribute.KeyValue) *MetricsExporter {
	meter := provider.Meter("github.com/kralicky/totem/metrics")

	rxBytes, err := meter.Int64Counter("stream_receive_bytes",
		metric.WithDescription("Total number of bytes received on a stream"))
	if err != nil {
		panic(err)
	}

	txBytes, err := meter.Int64Counter("stream_transmit_bytes",
		metric.WithDescription("Total number of bytes transmitted on a stream"))
	if err != nil {
		panic(err)
	}

	rxRpc, err := meter.Int64Counter("stream_receive_rpcs",
		metric.WithDescription("Total number of requests and replies received on a stream"))
	if err != nil {
		panic(err)
	}

	txRpc, err := meter.Int64Counter("stream_transmit_rpcs",
		metric.WithDescription("Total number of requests and replies transmitted on a stream"))
	if err != nil {
		panic(err)
	}

	svcRxLatency, err := meter.Int64Histogram("stream_local_service_latency",
		metric.WithDescription("Incoming RPC request-response latency for services handled locally on a stream"),
		metric.WithUnit("μs"),
	)
	if err != nil {
		panic(err)
	}

	svcTxLatency, err := meter.Int64Histogram("stream_remote_service_latency",
		metric.WithDescription("Outgoing RPC request-response latency for services handled remotely on a stream"),
		metric.WithUnit("μs"),
	)
	if err != nil {
		panic(err)
	}

	return &MetricsExporter{
		rxBytesCollector:      rxBytes,
		txBytesCollector:      txBytes,
		rxRpcCollector:        rxRpc,
		txRpcCollector:        txRpc,
		svcRxLatencyCollector: svcRxLatency,
		svcTxLatencyCollector: svcTxLatency,
		staticAttrs:           staticAttrs,
	}
}

func (m *MetricsExporter) TrackRxBytes(service, method string, count int64) {
	if m == nil {
		return
	}
	attrs := append(m.staticAttrs,
		attribute.Key("service").String(service),
		attribute.Key("method").String(method),
	)
	m.rxRpcCollector.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	m.rxBytesCollector.Add(context.Background(), count, metric.WithAttributes(attrs...))
}

func (m *MetricsExporter) TrackTxBytes(service, method string, count int64) {
	if m == nil {
		return
	}
	attrs := append(m.staticAttrs,
		attribute.Key("service").String(service),
		attribute.Key("method").String(method),
	)
	m.txRpcCollector.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	m.txBytesCollector.Add(context.Background(), count, metric.WithAttributes(attrs...))
}

func (m *MetricsExporter) TrackSvcRxLatency(service, method string, latency time.Duration) {
	if m == nil {
		return
	}
	attrs := append(m.staticAttrs,
		attribute.Key("service").String(service),
		attribute.Key("method").String(method),
	)
	m.svcRxLatencyCollector.Record(context.Background(), latency.Microseconds(), metric.WithAttributes(attrs...))
}

func (m *MetricsExporter) TrackSvcTxLatency(service, method string, latency time.Duration) {
	if m == nil {
		return
	}
	attrs := append(m.staticAttrs,
		attribute.Key("service").String(service),
		attribute.Key("method").String(method),
	)
	m.svcTxLatencyCollector.Record(context.Background(), latency.Microseconds(), metric.WithAttributes(attrs...))
}
