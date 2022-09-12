package totem

import (
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/metadata"
)

func (md *MD) ToMD() metadata.MD {
	m := map[string][]string{}
	for k, v := range md.GetData() {
		m[k] = v.Items
	}
	return metadata.MD(m)
}

func FromMD(md metadata.MD) *MD {
	m := MD{
		Data: map[string]*MDValues{},
	}
	for k, v := range md {
		m.Data[k] = &MDValues{
			Items: v,
		}
	}
	return &m
}

func (md *MD) KV() []attribute.KeyValue {
	kv := []attribute.KeyValue{}
	for k, v := range md.GetData() {
		if len(v.Items) == 1 {
			kv = append(kv, attribute.String(k, v.Items[0]))
		} else {
			kv = append(kv, attribute.StringSlice(k, v.Items))
		}
	}
	return kv
}
