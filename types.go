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

// stream is a type constraint for the Stream interface, used to enable comparison
// operators for objects instantiated using type parameters.
type stream interface {
	comparable
	Stream
}
