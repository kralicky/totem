package totem

import (
	"fmt"
	"sync"

	"go.uber.org/atomic"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/status"
)

type streamHandler struct {
	stream      Stream
	count       *atomic.Uint64
	sendLock    sync.Mutex
	pendingRPCs map[uint64]chan *RPC
	pendingLock sync.RWMutex
	methods     map[string]MethodInvoker
	receiver    *recvWrapper
	kickOnce    sync.Once
}

// NewStreamHandler creates a new stream handler for the given stream and
// method set.
// There can be at most one stream handler per stream.
func newStreamHandler(stream Stream, methods map[string]MethodInvoker) *streamHandler {
	sh := &streamHandler{
		stream:      stream,
		count:       atomic.NewUint64(0),
		pendingRPCs: map[uint64]chan *RPC{},
		methods:     methods,
		receiver:    newRecvWrapper(stream),
	}
	return sh
}

func (sh *streamHandler) Request(m *RPC) <-chan *RPC {
	tag := sh.count.Inc()
	m.Tag = tag
	ch := make(chan *RPC, 1)

	sh.pendingLock.Lock()
	sh.pendingRPCs[m.Tag] = ch
	sh.pendingLock.Unlock()

	sh.sendLock.Lock()
	defer sh.sendLock.Unlock()
	if err := sh.stream.Send(m); err != nil {
		sh.Kick()
	}
	return ch
}

func (sh *streamHandler) Reply(tag uint64, data []byte) {
	sh.sendLock.Lock()
	defer sh.sendLock.Unlock()
	if err := sh.stream.Send(&RPC{
		Tag: tag,
		Content: &RPC_Response{
			Response: &Response{
				Response:    data,
				StatusProto: status.New(codes.OK, "").Proto(),
			},
		},
	}); err != nil {
		sh.Kick()
	}
}

func (sh *streamHandler) ReplyErr(tag uint64, err error) {
	sh.sendLock.Lock()
	defer sh.sendLock.Unlock()
	stat, _ := status.FromError(err)
	if err := sh.stream.Send(&RPC{
		Tag: tag,
		Content: &RPC_Response{
			Response: &Response{
				Response:    nil,
				StatusProto: stat.Proto(),
			},
		},
	}); err != nil {
		sh.Kick()
	}
}

func (sh *streamHandler) Kick() {
	sh.kickOnce.Do(func() {
		close(sh.receiver.kick)
	})
}

// Run will start the stream handler and block until the stream is finished.
// This function should only be called once.
func (sh *streamHandler) Run(ctx context.Context) error {
	var streamErr error
	sh.receiver.Start()
	ctx, span := Tracer().Start(ctx, "StreamHandler.Run")
	defer span.End()
	for {
		msg, err := sh.receiver.Recv()
		if err != nil {
			streamErr = err
			break
		}
		switch msg.Content.(type) {
		case *RPC_Request:
			// Received a request from the client
			method := msg.GetMethod()
			if m, ok := sh.methods[method]; ok {
				// Found a handler, call it
				go func(msg *RPC) {
					// very important to clone the message here, otherwise the tag
					// will be overwritten, and we need to preserve it to reply to
					// the original request
					response, err := m.Invoke(addTotemToContext(ctx), proto.Clone(msg).(*RPC))
					if err != nil {
						sh.ReplyErr(msg.Tag, err)
						return
					}
					sh.Reply(msg.Tag, response)
				}(msg)
			} else {
				sh.ReplyErr(msg.Tag, status.Errorf(codes.Unimplemented,
					"method %s not implemented", method))
			}
		case *RPC_Response:
			// Received a response from the server
			sh.pendingLock.Lock()
			future, ok := sh.pendingRPCs[msg.Tag]
			if !ok {
				return status.Error(codes.Internal,
					fmt.Sprintf("unexpected tag: %d", msg.Tag))
			}
			future <- msg
			delete(sh.pendingRPCs, msg.Tag)
			sh.pendingLock.Unlock()
		default:
			return fmt.Errorf("invalid content type")
		}
	}
	sh.pendingLock.Lock()
	defer sh.pendingLock.Unlock()
	for tag, future := range sh.pendingRPCs {
		future <- &RPC{
			Tag: tag,
			Content: &RPC_Response{
				Response: &Response{
					StatusProto: status.New(codes.Canceled, streamErr.Error()).Proto(),
				},
			},
		}
	}
	return streamErr
}
