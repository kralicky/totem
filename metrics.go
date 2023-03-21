package totem

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/sdk/metric"
)

type MetricsExporter struct {
	rxBytesCollector instrument.Int64Counter
	txBytesCollector instrument.Int64Counter

	rxRpcCollector instrument.Int64Counter
	txRpcCollector instrument.Int64Counter

	svcRxLatencyCollector instrument.Int64Histogram
	svcTxLatencyCollector instrument.Int64Histogram

	staticAttrs []attribute.KeyValue
}

// func (m *MetricsExporter) Inverted() *MetricsExporter {
// 	if m == nil {
// 		return nil
// 	}
// 	return &MetricsExporter{
// 		rxBytesCollector: m.txBytesCollector,
// 		txBytesCollector: m.rxBytesCollector,

// 		rxRpcCollector: m.txRpcCollector,
// 		txRpcCollector: m.rxRpcCollector,

// 		svcRxLatencyCollector: m.svcRxLatencyCollector,
// 		svcTxLatencyCollector: m.svcTxLatencyCollector,

// 		staticAttrs: m.staticAttrs,
// 	}
// }

func NewMetricsExporter(reader metric.Reader, staticAttrs ...attribute.KeyValue) *MetricsExporter {
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("github.com/kralicky/totem/metrics")

	rxBytes, err := meter.Int64Counter("stream_receive_bytes",
		instrument.WithDescription("Total number of bytes received on a stream"))
	if err != nil {
		panic(err)
	}

	txBytes, err := meter.Int64Counter("stream_transmit_bytes",
		instrument.WithDescription("Total number of bytes transmitted on a stream"))
	if err != nil {
		panic(err)
	}

	rxRpc, err := meter.Int64Counter("stream_receive_rpcs",
		instrument.WithDescription("Total number of requests and replies received on a stream"))
	if err != nil {
		panic(err)
	}

	txRpc, err := meter.Int64Counter("stream_transmit_rpcs",
		instrument.WithDescription("Total number of requests and replies transmitted on a stream"))
	if err != nil {
		panic(err)
	}

	svcRxLatency, err := meter.Int64Histogram("stream_local_service_latency",
		instrument.WithDescription("Incoming RPC request-response latency for services handled locally on a stream"),
		instrument.WithUnit("μs"),
	)
	if err != nil {
		panic(err)
	}

	svcTxLatency, err := meter.Int64Histogram("stream_remote_service_latency",
		instrument.WithDescription("Outgoing RPC request-response latency for services handled remotely on a stream"),
		instrument.WithUnit("μs"),
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
	m.rxRpcCollector.Add(context.Background(), 1, attrs...)
	m.rxBytesCollector.Add(context.Background(), count, attrs...)
}

func (m *MetricsExporter) TrackTxBytes(service, method string, count int64) {
	if m == nil {
		return
	}
	attrs := append(m.staticAttrs,
		attribute.Key("service").String(service),
		attribute.Key("method").String(method),
	)
	m.txRpcCollector.Add(context.Background(), 1, attrs...)
	m.txBytesCollector.Add(context.Background(), count, attrs...)
}

func (m *MetricsExporter) TrackSvcRxLatency(service, method string, latency time.Duration) {
	if m == nil {
		return
	}
	attrs := append(m.staticAttrs,
		attribute.Key("service").String(service),
		attribute.Key("method").String(method),
	)
	m.svcRxLatencyCollector.Record(context.Background(), latency.Microseconds(), attrs...)
}

func (m *MetricsExporter) TrackSvcTxLatency(service, method string, latency time.Duration) {
	if m == nil {
		return
	}
	attrs := append(m.staticAttrs,
		attribute.Key("service").String(service),
		attribute.Key("method").String(method),
	)
	m.svcTxLatencyCollector.Record(context.Background(), latency.Microseconds(), attrs...)
}
