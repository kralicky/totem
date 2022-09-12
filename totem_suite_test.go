package totem_test

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	. "github.com/kralicky/totem/test"
)

func TestTotem(t *testing.T) {
	SetDefaultEventuallyTimeout(5 * time.Second)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Totem Suite")
}

var _ = BeforeSuite(func() {
	res, err := resource.New(context.Background(),
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("totem_test"),
		),
	)
	Expect(err).NotTo(HaveOccurred())
	opts := []tracesdk.TracerProviderOption{
		tracesdk.WithResource(res),
	}

	exp, err := jaeger.New(jaeger.WithCollectorEndpoint())
	Expect(err).NotTo(HaveOccurred())
	opts = append(opts, tracesdk.WithBatcher(exp))

	tp := tracesdk.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())

	DeferCleanup(func() {
		tp.ForceFlush(context.Background())
	})
})

type testCase struct {
	ServerHandler func(stream Test_TestStreamServer) error
	ClientHandler func(stream Test_TestStreamClient) error

	listener net.Listener
}

type testServer struct {
	UnsafeTestServer
	testCase *testCase
	wg       sync.WaitGroup
}

func (ts *testServer) TestStream(stream Test_TestStreamServer) error {
	defer ts.wg.Done()
	return ts.testCase.ServerHandler(stream)
}

func (tc *testCase) Run(until ...chan struct{}) {
	var err error
	tc.listener, err = net.Listen("tcp4", "localhost:0")
	Expect(err).NotTo(HaveOccurred())
	srv := testServer{
		testCase: tc,
		wg:       sync.WaitGroup{},
	}
	srv.wg.Add(2)
	grpcServer := grpc.NewServer()
	RegisterTestServer(grpcServer, &srv)
	go grpcServer.Serve(tc.listener)
	conn := tc.Dial()
	defer conn.Close()
	client := NewTestClient(conn)
	stream, _ := client.TestStream(context.Background())
	go func() {
		defer GinkgoRecover()
		err := tc.ClientHandler(stream)
		if err != nil {
			if status.Code(err) != codes.Canceled && !errors.Is(err, io.EOF) {
				Fail(err.Error())
			}
		}
		srv.wg.Done()
	}()

	if len(until) > 0 {
		for _, c := range until {
			<-c
		}
	} else {
		done := make(chan struct{})
		go func() {
			defer close(done)
			srv.wg.Wait()
		}()
		Eventually(done).Should(BeClosed())
	}
}

func (tc *testCase) RunServerOnly() {
	var err error
	tc.listener, err = net.Listen("tcp4", "localhost:0")
	Expect(err).NotTo(HaveOccurred())
	srv := testServer{
		testCase: tc,
		wg:       sync.WaitGroup{},
	}
	srv.wg.Add(1)
	grpcServer := grpc.NewServer()
	RegisterTestServer(grpcServer, &srv)
	grpcServer.Serve(tc.listener)
}

func (tc *testCase) AsyncRunServerOnly() {
	var err error
	tc.listener, err = net.Listen("tcp4", "localhost:0")
	Expect(err).NotTo(HaveOccurred())
	srv := testServer{
		testCase: tc,
		wg:       sync.WaitGroup{},
	}
	srv.wg.Add(1)
	grpcServer := grpc.NewServer()
	RegisterTestServer(grpcServer, &srv)
	go grpcServer.Serve(tc.listener)
}

func (tc *testCase) Dial() *grpc.ClientConn {
	conn, err := grpc.DialContext(context.Background(), tc.listener.Addr().String(),
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	Expect(err).NotTo(HaveOccurred())
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
	UnsafeIncrementServer
	requestLimiter
}

func (s *incrementServer) Inc(ctx context.Context, n *Number) (*Number, error) {
	defer s.requestLimiter.Tick()
	return &Number{
		Value: n.Value + 1,
	}, nil
}

type decrementServer struct {
	UnsafeDecrementServer
	requestLimiter
}

func (s *decrementServer) Dec(ctx context.Context, n *Number) (*Number, error) {
	defer s.requestLimiter.Tick()
	return &Number{
		Value: n.Value - 1,
	}, nil
}

type hashServer struct {
	UnsafeHashServer
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
	UnsafeAddSubServer
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

type multiplyServer struct {
	UnsafeMultiplyServer
	requestLimiter
}

func (s *multiplyServer) Mul(ctx context.Context, op *Operands) (*Number, error) {
	defer s.requestLimiter.Tick()
	return &Number{
		Value: int64(op.A) * int64(op.B),
	}, nil
}
