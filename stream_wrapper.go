package totem

import (
	"context"
	"fmt"
	"io"
	sync "sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

var ErrConnClosing = status.Error(codes.Unavailable, "transport is closing")
var ErrIllegalHeaderWrite = status.Error(codes.Internal, "SendHeader called multiple times")

type localServerStreamWrapper struct {
	ctx          context.Context
	req          *RPC
	headers      metadata.MD
	trailers     metadata.MD
	wroteHeaders atomic.Bool
	writer       chan<- *RPC
	closedWriter chan struct{}
}

func newLocalServerStreamWrapper(ctx context.Context, req *RPC, writer chan<- *RPC) *localServerStreamWrapper {
	return &localServerStreamWrapper{
		ctx:          ctx,
		req:          req,
		writer:       writer,
		closedWriter: make(chan struct{}),
	}
}

func (s *localServerStreamWrapper) Close() (trailers metadata.MD) {
	close(s.closedWriter)
	return s.trailers
}

// Context implements grpc.ServerStream.
func (s *localServerStreamWrapper) Context() context.Context {
	return s.ctx
}

// RecvMsg implements grpc.ServerStream.
func (s *localServerStreamWrapper) RecvMsg(m interface{}) error {
	// unmarshal the request into m
	switch m := m.(type) {
	case *RPC:
		m.Content = s.req.Content
	case protoadapt.MessageV2:
		if err := proto.Unmarshal(s.req.GetRequest(), m); err != nil {
			return fmt.Errorf("[totem] malformed request: %w", err)
		}
	case protoadapt.MessageV1:
		m2 := protoadapt.MessageV2Of(m)
		if err := proto.Unmarshal(s.req.GetRequest(), m2); err != nil {
			return fmt.Errorf("[totem] malformed request: %w", err)
		}
	default:
		panic(fmt.Sprintf("[totem] unsupported request type: %T", m))
	}
	return nil
}

// SendHeader implements grpc.ServerStream.
func (s *localServerStreamWrapper) SendHeader(md metadata.MD) error {
	if err := s.SetHeader(md); err != nil {
		return err
	}
	if s.wroteHeaders.CompareAndSwap(false, true) {
		select {
		case s.writer <- &RPC{
			Content:  &RPC_ServerStreamMsg{},
			Metadata: FromMD(s.headers),
		}:
			s.headers = nil
		case <-s.closedWriter:
			return ErrConnClosing
		}
	} else {
		return ErrIllegalHeaderWrite
	}
	return nil
}

// SendMsg implements grpc.ServerStream.
func (s *localServerStreamWrapper) SendMsg(m interface{}) error {
	bytes, err := proto.Marshal(protoimpl.X.ProtoMessageV2Of(m))
	if err != nil {
		return err
	}
	rpc := &RPC{
		Content: &RPC_ServerStreamMsg{
			ServerStreamMsg: &ServerStreamMessage{
				Response: bytes,
			},
		},
	}
	if s.wroteHeaders.CompareAndSwap(false, true) {
		rpc.Metadata = FromMD(s.headers)
		s.headers = nil
	}
	select {
	case s.writer <- rpc:
		return nil
	case <-s.closedWriter:
		return ErrConnClosing
	}
}

// SetHeader implements grpc.ServerStream.
func (s *localServerStreamWrapper) SetHeader(md metadata.MD) error {
	if md.Len() == 0 {
		return nil
	}
	if err := validateMetadata(md); err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	// set the header data, which will be sent when SendHeader is called or the
	// first message is sent
	s.headers = metadata.Join(s.headers, md)
	return nil
}

// SetTrailer implements grpc.ServerStream.
func (s *localServerStreamWrapper) SetTrailer(md metadata.MD) {
	if md.Len() == 0 {
		return
	}
	// the actual implementation sets trailers regardless of whether they
	// pass validation, so we do the same
	s.trailers = metadata.Join(s.trailers, md)
}

var _ grpc.ServerStream = (*localServerStreamWrapper)(nil)

type serverStreamClientWrapper struct {
	ctx              context.Context
	controller       *StreamController
	streamName       string
	methodName       string
	bufferedFirstMsg []byte
	replyC           <-chan *RPC
	writeHeadersOnce sync.Once
	headerC          chan metadata.MD
	recvHeadersOnce  func() (metadata.MD, error)
	trailer          metadata.MD

	onFinishedOnce sync.Once
	onFinish       []func(error)
}

func newServerStreamClientWrapper(
	ctx context.Context,
	ctrl *StreamController, streamName, methodName string,
	opts ...grpc.CallOption,
) *serverStreamClientWrapper {
	hc := make(chan metadata.MD, 1)
	cs := &serverStreamClientWrapper{
		ctx:        ctx,
		controller: ctrl,
		streamName: streamName,
		methodName: methodName,
		headerC:    hc,
		recvHeadersOnce: sync.OnceValues(func() (metadata.MD, error) {
			select {
			case md := <-hc:
				return md, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}),
	}
	for _, opt := range opts {
		if o, ok := opt.(grpc.OnFinishCallOption); ok {
			cs.onFinish = append(cs.onFinish, o.OnFinish)
		}
	}
	return cs
}

// CloseSend implements grpc.ClientStream.
func (cs *serverStreamClientWrapper) CloseSend() error {
	clientHeaders, ok := metadata.FromOutgoingContext(cs.ctx)
	if !ok {
		clientHeaders = metadata.New(nil)
	}
	cs.replyC = cs.controller.Request(cs.ctx, &RPC{
		ServiceName: cs.streamName,
		MethodName:  cs.methodName,
		Content: &RPC_Request{
			Request: cs.bufferedFirstMsg,
		},
		Metadata: FromMD(clientHeaders),
	})
	return nil
}

// Context implements grpc.ClientStream.
func (cs *serverStreamClientWrapper) Context() context.Context {
	return cs.ctx
}

// Header implements grpc.ClientStream.
func (cs *serverStreamClientWrapper) Header() (metadata.MD, error) {
	select {
	case md := <-cs.headerC:
		return md, nil
	case <-cs.ctx.Done():
		return nil, cs.ctx.Err()
	}
}

func (cs *serverStreamClientWrapper) finish(err error) {
	cs.onFinishedOnce.Do(func() {
		for _, fn := range cs.onFinish {
			fn(err)
		}
	})
}

// RecvMsg implements grpc.ClientStream.
func (cs *serverStreamClientWrapper) RecvMsg(m interface{}) error {
	select {
	case rpc, ok := <-cs.replyC:
		if !ok {
			cs.finish(nil)
			return io.EOF
		}
		switch rpc.Content.(type) {
		case *RPC_ServerStreamMsg:
			msg := rpc.GetServerStreamMsg()
			switch m := m.(type) {
			case *RPC:
				m.Content = &RPC_ServerStreamMsg{
					ServerStreamMsg: msg,
				}
			case protoadapt.MessageV2:
				cs.writeHeadersOnce.Do(func() { cs.headerC <- msg.Headers.ToMD() })
				if err := proto.Unmarshal(msg.GetResponse(), m); err != nil {
					return fmt.Errorf("[totem] malformed response: %w", err)
				}
			case protoadapt.MessageV1:
				cs.writeHeadersOnce.Do(func() { cs.headerC <- msg.Headers.ToMD() })
				m2 := protoadapt.MessageV2Of(m)
				if err := proto.Unmarshal(msg.GetResponse(), m2); err != nil {
					return fmt.Errorf("[totem] malformed response: %w", err)
				}
			default:
				panic(fmt.Sprintf("[totem] unsupported response type: %T", m))
			}
		case *RPC_Response:
			resp := rpc.GetResponse()
			status := resp.GetStatus()
			if err := status.Err(); err != nil {
				cs.finish(err)
				return err
			}
			switch m := m.(type) {
			case *RPC:
				m.Content = &RPC_Response{
					Response: resp,
				}
			case protoadapt.MessageV2:
				if rpc.Metadata != nil {
					cs.trailer = rpc.Metadata.ToMD()
				}
				if err := proto.Unmarshal(resp.GetResponse(), m); err != nil {
					return fmt.Errorf("[totem] malformed response: %w", err)
				}
			case protoadapt.MessageV1:
				if rpc.Metadata != nil {
					cs.trailer = rpc.Metadata.ToMD()
				}
				m2 := protoadapt.MessageV2Of(m)
				if err := proto.Unmarshal(resp.GetResponse(), m2); err != nil {
					return fmt.Errorf("[totem] malformed response: %w", err)
				}
			default:
				panic(fmt.Sprintf("[totem] unsupported response type: %T", m))
			}
		default:
			return fmt.Errorf("[totem] unexpected response type: %T", rpc.Content)
		}
	case <-cs.ctx.Done():
		return cs.ctx.Err()
	}
	return nil
}

// SendMsg implements grpc.ClientStream.
func (cs *serverStreamClientWrapper) SendMsg(m interface{}) error {
	if cs.bufferedFirstMsg != nil {
		return status.Errorf(codes.Internal, "unexpected call to SendMsg on server-streaming RPC")
	}
	var err error
	cs.bufferedFirstMsg, err = proto.Marshal(protoimpl.X.ProtoMessageV2Of(m))
	if err != nil {
		return err
	}
	return nil
}

// Trailer implements grpc.ClientStream.
func (cs *serverStreamClientWrapper) Trailer() metadata.MD {
	return cs.trailer
}

var _ grpc.ClientStream = (*serverStreamClientWrapper)(nil)
