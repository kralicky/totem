package totem

import grpc "google.golang.org/grpc"

type ServerStream interface {
	Stream
	grpc.ServerStream
}

type ClientStream interface {
	Stream
	grpc.ClientStream
}

type Stream interface {
	Send(*RPC) error
	Recv() (*RPC, error)
}
