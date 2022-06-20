package totem_test

import (
	context "context"
	"crypto/sha1"
	"encoding/hex"
	"net"
	"sync"
	"testing"
	"time"

	. "github.com/kralicky/totem/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/atomic"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestTotem(t *testing.T) {
	SetDefaultEventuallyTimeout(1 * time.Hour)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Totem Suite")
}

type testCase struct {
	ServerHandler func(stream Test_TestStreamServer) error
	ClientHandler func(stream Test_TestStreamClient) error

	listener net.Listener
}

type testServer struct {
	UnimplementedTestServer
	testCase *testCase
	wg       sync.WaitGroup
	called   *atomic.Bool
}

func (ts *testServer) TestStream(stream Test_TestStreamServer) error {
	if !ts.called.CAS(false, true) {
		panic("TestStream called twice")
	}
	defer ts.wg.Done()
	defer GinkgoRecover()
	return ts.testCase.ServerHandler(stream)
}

func (tc *testCase) Run() {
	defer GinkgoRecover()
	var err error
	tc.listener, err = net.Listen("tcp4", "localhost:0")
	if err != nil {
		panic(err)
	}
	srv := testServer{
		called:   atomic.NewBool(false),
		testCase: tc,
		wg:       sync.WaitGroup{},
	}
	srv.wg.Add(2)
	grpcServer := grpc.NewServer()
	RegisterTestServer(grpcServer, &srv)
	go func() {
		defer GinkgoRecover()
		err := grpcServer.Serve(tc.listener)
		Expect(err).NotTo(HaveOccurred())
	}()
	conn := tc.Dial()
	defer conn.Close()
	client := NewTestClient(conn)
	stream, _ := client.TestStream(context.Background())
	go func() {
		defer GinkgoRecover()
		err := tc.ClientHandler(stream)
		Expect(err).NotTo(HaveOccurred())
		srv.wg.Done()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.wg.Wait()
	}()
	Eventually(done).Should(BeClosed())
}

func (tc *testCase) RunServerOnly() {
	defer GinkgoRecover()
	var err error
	tc.listener, err = net.Listen("tcp4", "localhost:0")
	if err != nil {
		panic(err)
	}
	srv := testServer{
		called:   atomic.NewBool(false),
		testCase: tc,
		wg:       sync.WaitGroup{},
	}
	srv.wg.Add(1)
	grpcServer := grpc.NewServer()
	RegisterTestServer(grpcServer, &srv)
	grpcServer.Serve(tc.listener)
}

func (tc *testCase) Dial() *grpc.ClientConn {
	conn, err := grpc.DialContext(context.Background(), tc.listener.Addr().String(), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	return conn
}

// test service implementations

type requestLimiter struct {
	mu        sync.Mutex
	remaining int
	done      chan struct{}
}

func (l *requestLimiter) LimitRequests(n int) <-chan struct{} {
	done := make(chan struct{})
	l.remaining = n
	l.done = done
	return done
}

func (l *requestLimiter) Tick() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.done == nil {
		return
	}
	l.remaining--
	if l.remaining == 0 {
		close(l.done)
	}
}

type incrementServer struct {
	UnimplementedIncrementServer
	requestLimiter
}

func (s *incrementServer) Inc(ctx context.Context, n *Number) (*Number, error) {
	defer s.requestLimiter.Tick()
	return &Number{
		Value: n.Value + 1,
	}, nil
}

type decrementServer struct {
	UnimplementedDecrementServer
	requestLimiter
}

func (s *decrementServer) Dec(ctx context.Context, n *Number) (*Number, error) {
	defer s.requestLimiter.Tick()
	return &Number{
		Value: n.Value - 1,
	}, nil
}

type hashServer struct {
	UnimplementedHashServer
	requestLimiter
}

func (s *hashServer) Hash(ctx context.Context, str *String) (*String, error) {
	defer s.requestLimiter.Tick()
	hash := sha1.Sum([]byte(str.Str))
	return &String{
		Str: hex.EncodeToString(hash[:]),
	}, nil
}

type addSubServer struct {
	UnimplementedAddSubServer
	requestLimiter
}

func (s *addSubServer) Add(ctx context.Context, op *Operands) (*Number, error) {
	defer s.requestLimiter.Tick()
	return &Number{
		Value: int64(op.A) + int64(op.B),
	}, nil
}

func (s *addSubServer) Sub(ctx context.Context, op *Operands) (*Number, error) {
	defer s.requestLimiter.Tick()
	return &Number{
		Value: int64(op.A) - int64(op.B),
	}, nil
}

type errorServer struct {
	UnimplementedErrorServer
	requestLimiter
}

func (s *errorServer) Error(ctx context.Context, err *ErrorRequest) (*emptypb.Empty, error) {
	defer s.requestLimiter.Tick()
	if err.ReturnError {
		return nil, status.Error(codes.Aborted, "error")
	} else {
		return &emptypb.Empty{}, nil
	}
}
