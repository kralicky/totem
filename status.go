package totem

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (r *Response) GetStatus() *status.Status {
	if r == nil || r.StatusProto == nil {
		return status.New(codes.Unknown, codes.Unknown.String())
	}
	return status.FromProto(r.GetStatusProto())
}
