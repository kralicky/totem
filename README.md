Totem is a Go library that can turn a single gRPC stream into bidirectional unary gRPC servers.

## Background

Streaming RPCs enable several useful design patterns for client-server connections that can't be done with unary RPCs. For example, keeping track of long-lived client connections, and sending server-initiated requests to such clients. However, implementing bidirectional messaging over streams can quickly become very complicated for non-trivial use cases. Totem enables these design patterns and abstracts away the underlying stream, allowing you to implement your streaming RPC in terms of simpler unary RPCs.

## Examples

See the [examples](examples/) directory for example code.