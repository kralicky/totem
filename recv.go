package totem

import (
	"fmt"
	"runtime/debug"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stacktrace struct {
	stack string
	once  sync.Once
}

func (st *stacktrace) Load() {
	st.once.Do(func() {
		st.stack = string(debug.Stack())
	})
}

func (st *stacktrace) Print() {
	fmt.Println("originally called from:\n" + string(st.stack))
}

type recvPayload struct {
	RPC *RPC
	Err error
}

type recvWrapper struct {
	stream   Stream
	c        chan recvPayload
	kick     chan struct{}
	runLock  sync.Mutex
	recvLock sync.Mutex

	runStack         stacktrace
	recvStack        stacktrace
	constructorStack stacktrace
}

func (w *recvWrapper) Start() chan error {
	if !w.runLock.TryLock() {
		w.runStack.Print()
		panic("attempted to call recvWrapper.Run twice")
	}
	w.runStack.Load()
	c := make(chan error)
	go func() {
		defer w.runLock.Unlock()
		for {
			rpc, err := w.stream.Recv()
			w.c <- recvPayload{
				RPC: rpc,
				Err: err,
			}
			if err != nil {
				c <- err
				return
			}
		}
	}()
	return c
}

func (w *recvWrapper) Recv() (*RPC, error) {
	if !w.recvLock.TryLock() {
		w.recvStack.Print()
		panic("attempted to call recvWrapper.Recv twice in separate goroutines")
	}
	w.recvStack.Load()
	defer w.recvLock.Unlock()
	select {
	case payload := <-w.c:
		return payload.RPC, payload.Err
	case <-w.kick:
		return nil, status.Error(codes.Canceled, "kicked")
	}
}

var gKnownStreams = map[Stream]*stacktrace{}
var gStreamLock sync.Mutex

func newRecvWrapper(stream Stream) *recvWrapper {
	gStreamLock.Lock()
	defer gStreamLock.Unlock()
	if st, ok := gKnownStreams[stream]; ok {
		st.Print()
		panic("attempted to create two recvWrappers for the same stream")
	}
	st := &stacktrace{}
	st.Load()
	gKnownStreams[stream] = st
	return &recvWrapper{
		stream: stream,
		c:      make(chan recvPayload, 256),
		kick:   make(chan struct{}),
	}
}
