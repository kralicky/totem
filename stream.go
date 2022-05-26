package totem

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recvPayload struct {
	RPC *RPC
	Err error
}

type recvWrapper struct {
	stream Stream
	c      chan recvPayload
	kick   chan struct{}
}

func (w *recvWrapper) Run() {
	for {
		rpc, err := w.stream.Recv()
		w.c <- recvPayload{
			RPC: rpc,
			Err: err,
		}
		if err != nil {
			return
		}
	}
}

var ErrStreamClosed = errors.New("stream closed")
var ErrKicked = errors.New("kicked")

func (w *recvWrapper) Recv() (*RPC, error) {
	select {
	case payload := <-w.c:
		return payload.RPC, payload.Err
	case <-w.kick:
		return nil, ErrKicked
	}
}

func newRecvWrapper(stream Stream) *recvWrapper {
	return &recvWrapper{
		stream: stream,
		c:      make(chan recvPayload, 256),
		kick:   make(chan struct{}),
	}
}

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

func newStreamHandler(stream Stream, methods map[string]MethodInvoker) *streamHandler {
	return &streamHandler{
		stream:      stream,
		count:       atomic.NewUint64(0),
		pendingRPCs: map[uint64]chan *RPC{},
		methods:     methods,
		receiver:    newRecvWrapper(stream),
	}
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
				Response: data,
			},
		},
	}); err != nil {
		sh.Kick()
	}
}

func (sh *streamHandler) ReplyErr(tag uint64, err error) {
	sh.sendLock.Lock()
	defer sh.sendLock.Unlock()
	if err := sh.stream.Send(&RPC{
		Tag: tag,
		Content: &RPC_Response{
			Response: &Response{
				Error: []byte(err.Error()),
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

func (sh *streamHandler) Run() error {
	var streamErr error
	go sh.receiver.Run()
	ctx, ca := context.WithCancel(context.Background())
	defer ca()
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
				req := msg.GetRequest()
				if req == nil {
					sh.ReplyErr(msg.Tag, status.Error(codes.InvalidArgument,
						"request is nil"))
					continue
				}
				go func() {
					response, err := m.Invoke(addTotemToContext(ctx), req)
					if err != nil {
						sh.ReplyErr(msg.Tag, err)
						return
					}

					sh.Reply(msg.Tag, response)
				}()
			} else {
				sh.ReplyErr(msg.Tag, status.Errorf(codes.Unimplemented,
					"method %s not implemented", method))
			}
		case *RPC_Response:
			// Received a response from the server
			sh.pendingLock.RLock()
			future, ok := sh.pendingRPCs[msg.Tag]
			sh.pendingLock.RUnlock()
			if !ok {
				return status.Error(codes.Internal,
					fmt.Sprintf("unexpected tag: %d", msg.Tag))
			}
			future <- msg
			sh.pendingLock.Lock()
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
					Error: []byte(streamErr.Error()),
				},
			},
		}
	}
	return streamErr
}
