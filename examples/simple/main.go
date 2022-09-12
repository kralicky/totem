package main

import (
	context "context"
	"fmt"
	"net"

	totem "github.com/kralicky/totem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// Implementation of the Example service
type exampleServer struct {
	UnimplementedExampleServer
}

// Implementation of the Stream RPC.
// The lifetime of the stream is controlled by this function. Once it returns,
// the stream is closed. The `stream` parameter can be used to send and receive
// messages to/from the client on the other end of the stream.
func (e *exampleServer) Stream(stream Example_StreamServer) error {
	// We will first create a totem server which will serve unary gRPC services
	// on the stream.
	totemServer, err := totem.NewServer(stream)
	if err != nil {
		return err
	}
	// The server implements grpc.ServiceRegistrar, so we can register services
	// to it the same way we would for a grpc server. The services registered to
	// the server are the ones the server wants to serve using Totem over
	// this stream. The real grpc server (the one handling this stream) does not
	// register these services.
	RegisterHelloServer(totemServer, &helloServer{})
	// Lastly, we call Serve() which will take over the stream. Serve() starts the
	// totem server in the background, and returns a grpc.ClientConnInterface
	// which can be used to create grpc clients to services registered by another
	// Totem server on the other end of this stream. Serve() also returns a
	// <-chan error (which we are ignoring for this example). An error will be
	// written to this channel if an error occurs on the stream (e.g. the client
	// went away, connection lost, etc). If you want to keep the stream open
	// long-term, you should wait for the error channel to be written to before
	// returning out of this function.
	clientConn, errC := totemServer.Serve()

	// The rest of this code is plain grpc. We create a client to the Hello
	// service on the other end of the stream, and call a unary RPC.
	helloClient := NewHelloClient(clientConn)
	resp, err := helloClient.Hello(context.Background(), &HelloRequest{
		Name: "server",
	})
	if err != nil {
		return err
	}

	// Will print "Client responded: Hello, server"
	fmt.Println("Client responded: " + resp.GetMessage())

	_ = errC // ignore this for the example (see above)
	return nil
}

// Implementation of the Hello service
type helloServer struct {
	UnimplementedHelloServer
}

// There is nothing special about the Hello implementation - it is just a
// regular unary RPC.
func (h *helloServer) Hello(ctx context.Context, req *HelloRequest) (*HelloResponse, error) {
	// We can check if we're inside a Totem stream using the context:
	if totem.CheckContext(ctx) {
		// inside a Totem stream (not a real grpc server)
	}
	return &HelloResponse{
		Message: "Hello, " + req.GetName(),
	}, nil
}

func main() {
	// Start the server using bufconn
	listener := bufconn.Listen(1024)
	server := grpc.NewServer()
	RegisterExampleServer(server, &exampleServer{})
	go server.Serve(listener)

	// This dial code is bufconn-specific
	conn, _ := grpc.DialContext(context.Background(), "bufconn",
		grpc.WithContextDialer(func(c context.Context, s string) (net.Conn, error) {
			return listener.Dial()
		}), grpc.WithInsecure())

	// Create a new client as usual
	exampleClient := NewExampleClient(conn)
	// Start the streaming RPC
	stream, _ := exampleClient.Stream(context.Background())

	// This is the client side of the stream. Similar to the server code above,
	// we create a totem server for the stream and register the Hello service to it.
	totemServer, err := totem.NewServer(stream)
	if err != nil {
		panic(err)
	}
	RegisterHelloServer(totemServer, &helloServer{})

	// Call totem.Handle to take over the client end of the stream. Like above,
	// this returns a grpc.ClientConnInterface which can be used to create grpc
	// clients to services registered by the Totem server on the server end
	// of the stream.
	clientConn, _ := totemServer.Serve()

	// As before, create a client to the Hello service and call a unary RPC. This
	// code is plain grpc.
	helloClient := NewHelloClient(clientConn)
	resp, _ := helloClient.Hello(context.Background(), &HelloRequest{
		Name: "client",
	})
	fmt.Println("Server responded: " + resp.GetMessage())

	server.GracefulStop()
}
