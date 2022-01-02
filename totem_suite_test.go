package totem_test

import (
	context "context"
	"crypto/sha1"
	"encoding/hex"
	"net"
	"sync"
	"testing"

	. "github.com/kralicky/totem/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestTotem(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Totem Suite")
}

type testCase struct {
	ServerHandler func(stream Test_TestStreamServer) error
	ClientHandler func(stream Test_TestStreamClient) error
}

type testServer struct {
	UnimplementedTestServer
	testCase *testCase
	wg       sync.WaitGroup
}

func (ts *testServer) TestStream(stream Test_TestStreamServer) error {
	defer ts.wg.Done()
	defer GinkgoRecover()
	return ts.testCase.ServerHandler(stream)
}

func (tc *testCase) Run() {
	listener := bufconn.Listen(1024)
	srv := testServer{
		testCase: tc,
		wg:       sync.WaitGroup{},
	}
	srv.wg.Add(2)
	grpcServer := grpc.NewServer()
	RegisterTestServer(grpcServer, &srv)
	go func() {
		defer GinkgoRecover()
		err := grpcServer.Serve(listener)
		Expect(err).NotTo(HaveOccurred())
	}()
	conn, _ := grpc.DialContext(context.Background(), "bufconn",
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return listener.Dial()
		}), grpc.WithInsecure())
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
	if l.done == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
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

// type testServer[SS, SC, CS, CC any] struct {
// 	UnimplementedTestServer
// 	ctrl        *totem.Controller[Test_TestStreamServer]
// 	listener    *bufconn.Listener
// 	testHandler handlerFunc[SC]
// 	newClientFn func(grpc.ClientConnInterface) SC
// }

// type scenarioA = testServer[SToCServer, SToCClient, CToSServer, CToSClient]
// type scenarioB = testServer[CalculatorServer, CalculatorClient, CalculatorServer, CalculatorClient]

// var questions_answers = map[string]string{
// 	"How can entropy be reversed?":       "THERE IS AS YET INSUFFICIENT DATA FOR A MEANINGFUL ANSWER",
// 	"What is the meaning of life?":       "42",
// 	"What is the length of this string?": "34",
// }
// var errUnknown = errors.New("unknown")

// type handlerFunc[T any] func(T, <-chan error) error

// func newScenarioA(handler handlerFunc[SToCClient]) *scenarioA {
// 	listener := bufconn.Listen(1024)
// 	ts := &scenarioA{
// 		ctrl:        totem.NewController[Test_TestStreamServer](),
// 		testHandler: handler,
// 		listener:    listener,
// 		newClientFn: NewSToCClient,
// 	}
// 	RegisterCToSServer(ts.ctrl, &ctosServer{})
// 	grpcServer := grpc.NewServer()
// 	RegisterTestServer(grpcServer, ts)
// 	go grpcServer.Serve(listener)
// 	return ts
// }

// func newScenarioB(handler handlerFunc[CalculatorClient]) *scenarioB {
// 	listener := bufconn.Listen(1024)
// 	ts := &scenarioB{
// 		ctrl:        totem.NewController[Test_TestStreamServer](),
// 		testHandler: handler,
// 		listener:    listener,
// 		newClientFn: NewCalculatorClient,
// 	}
// 	RegisterCalculatorServer(ts.ctrl, &calculatorServer{})
// 	grpcServer := grpc.NewServer()
// 	RegisterTestServer(grpcServer, ts)
// 	go grpcServer.Serve(listener)
// 	return ts
// }

// func (t *testServer[SS, SC, CS, CC]) Dial() TestClient {
// 	conn, err := t.listener.Dial()
// 	Expect(err).NotTo(HaveOccurred())
// 	cc, err := grpc.DialContext(context.Background(), "bufconn",
// 		grpc.WithContextDialer(func(c context.Context, s string) (net.Conn, error) {
// 			return conn, nil
// 		}), grpc.WithInsecure())
// 	Expect(err).NotTo(HaveOccurred())
// 	return NewTestClient(cc)
// }

// func (t *testServer[SS, SC, CS, CC]) TestStream(stream Test_TestStreamServer) error {
// 	cc, errC := totem.Handle[CS](t.ctrl, stream)

// 	defer GinkgoRecover()
// 	client := t.newClientFn(cc)
// 	return t.testHandler(client, errC)
// }

// // ctosServer implements the server-side unary RPC CToSServer
// type ctosServer struct {
// 	UnimplementedCToSServer
// }

// func (s *ctosServer) AskServer(ctx context.Context, req *CRequest) (*SResponse, error) {
// 	defer GinkgoRecover()
// 	Expect(totem.CheckContext(ctx)).To(BeTrue())
// 	if answer, ok := questions_answers[req.Question]; ok {
// 		return &SResponse{
// 			Answer: answer,
// 		}, nil
// 	}
// 	return nil, errUnknown
// }

// // stocServer implements the client-side unary RPC SToCServer
// type stocServer struct {
// 	UnimplementedSToCServer
// }

// func (c *stocServer) AskClient(ctx context.Context, req *SRequest) (*CResponse, error) {
// 	defer GinkgoRecover()
// 	Expect(totem.CheckContext(ctx)).To(BeTrue())
// 	for k, v := range questions_answers {
// 		if v == req.Answer {
// 			return &CResponse{
// 				Question: k,
// 			}, nil
// 		}
// 	}
// 	return &CResponse{}, errUnknown
// }

// type calculatorServer struct {
// 	UnimplementedCalculatorServer
// }

// func (c *calculatorServer) Add(ctx context.Context, req *AddRequest) (*AddResponse, error) {
// 	return &AddResponse{
// 		Result: int64(req.Lhs) + int64(req.Rhs),
// 	}, nil
// }

// func runScenarioA(sf handlerFunc[SToCClient], cf handlerFunc[CToSClient]) {
// 	server := newScenarioA(sf)
// 	client := server.Dial()
// 	stream, err := client.TestStream(context.Background())
// 	Expect(err).NotTo(HaveOccurred())

// 	ctrl := totem.NewController(stream)
// 	RegisterSToCServer(ctrl, &stocServer{})
// 	clientConn, errC := totem.Handle[SToCServer](ctrl, stream)
// 	ctos := NewCToSClient(clientConn)

// 	Expect(cf(ctos, errC)).To(Succeed())
// }

// func runScenarioB(sf, cf handlerFunc[CalculatorClient]) {
// 	server := newScenarioB(sf)
// 	client := server.Dial()
// 	stream, err := client.TestStream(context.Background())
// 	Expect(err).NotTo(HaveOccurred())

// 	ctrl := totem.NewController(stream)
// 	RegisterCalculatorServer(ctrl, &calculatorServer{})
// 	clientConn, errC := totem.Handle[CalculatorServer](ctrl, stream)
// 	ctos := NewCalculatorClient(clientConn)

// 	Expect(cf(ctos, errC)).To(Succeed())
// }
