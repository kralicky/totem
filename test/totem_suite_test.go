package totem_test

import (
	context "context"
	"errors"
	"net"
	"testing"

	totem "github.com/kralicky/totem"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestTotem(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Totem Suite")
}

type testServer[SS, SC, CS, CC any] struct {
	UnimplementedTestServer
	ctrl        *totem.Controller[Test_TestStreamServer]
	listener    *bufconn.Listener
	testHandler handlerFunc[SC]
	newClientFn func(grpc.ClientConnInterface) SC
}

type scenarioA = testServer[SToCServer, SToCClient, CToSServer, CToSClient]
type scenarioB = testServer[CalculatorServer, CalculatorClient, CalculatorServer, CalculatorClient]

var questions_answers = map[string]string{
	"How can entropy be reversed?":       "THERE IS AS YET INSUFFICIENT DATA FOR A MEANINGFUL ANSWER",
	"What is the meaning of life?":       "42",
	"What is the length of this string?": "34",
}
var errUnknown = errors.New("unknown")

type handlerFunc[T any] func(T, <-chan error) error

func newScenarioA(handler handlerFunc[SToCClient]) *scenarioA {
	listener := bufconn.Listen(1024)
	ts := &scenarioA{
		ctrl:        totem.NewController[Test_TestStreamServer](),
		testHandler: handler,
		listener:    listener,
		newClientFn: NewSToCClient,
	}
	RegisterCToSServer(ts.ctrl, &ctosServer{})
	grpcServer := grpc.NewServer()
	RegisterTestServer(grpcServer, ts)
	go grpcServer.Serve(listener)
	return ts
}

func newScenarioB(handler handlerFunc[CalculatorClient]) *scenarioB {
	listener := bufconn.Listen(1024)
	ts := &scenarioB{
		ctrl:        totem.NewController[Test_TestStreamServer](),
		testHandler: handler,
		listener:    listener,
		newClientFn: NewCalculatorClient,
	}
	RegisterCalculatorServer(ts.ctrl, &calculatorServer{})
	grpcServer := grpc.NewServer()
	RegisterTestServer(grpcServer, ts)
	go grpcServer.Serve(listener)
	return ts
}

func (t *testServer[SS, SC, CS, CC]) Dial() TestClient {
	conn, err := t.listener.Dial()
	Expect(err).NotTo(HaveOccurred())
	cc, err := grpc.DialContext(context.Background(), "bufconn",
		grpc.WithContextDialer(func(c context.Context, s string) (net.Conn, error) {
			return conn, nil
		}), grpc.WithInsecure())
	Expect(err).NotTo(HaveOccurred())
	return NewTestClient(cc)
}

func (t *testServer[SS, SC, CS, CC]) TestStream(stream Test_TestStreamServer) error {
	cc, errC := totem.Handle[CS](t.ctrl, stream)

	defer GinkgoRecover()
	client := t.newClientFn(cc)
	return t.testHandler(client, errC)
}

// ctosServer implements the server-side unary RPC CToSServer
type ctosServer struct {
	UnimplementedCToSServer
}

func (s *ctosServer) AskServer(ctx context.Context, req *CRequest) (*SResponse, error) {
	defer GinkgoRecover()
	Expect(totem.CheckContext(ctx)).To(BeTrue())
	if answer, ok := questions_answers[req.Question]; ok {
		return &SResponse{
			Answer: answer,
		}, nil
	}
	return nil, errUnknown
}

// stocServer implements the client-side unary RPC SToCServer
type stocServer struct {
	UnimplementedSToCServer
}

func (c *stocServer) AskClient(ctx context.Context, req *SRequest) (*CResponse, error) {
	defer GinkgoRecover()
	Expect(totem.CheckContext(ctx)).To(BeTrue())
	for k, v := range questions_answers {
		if v == req.Answer {
			return &CResponse{
				Question: k,
			}, nil
		}
	}
	return &CResponse{}, errUnknown
}

type calculatorServer struct {
	UnimplementedCalculatorServer
}

func (c *calculatorServer) Add(ctx context.Context, req *AddRequest) (*AddResponse, error) {
	return &AddResponse{
		Result: int64(req.Lhs) + int64(req.Rhs),
	}, nil
}

func runScenarioA(sf handlerFunc[SToCClient], cf handlerFunc[CToSClient]) {
	server := newScenarioA(sf)
	client := server.Dial()
	stream, err := client.TestStream(context.Background())
	Expect(err).NotTo(HaveOccurred())

	ctrl := totem.NewController(stream)
	RegisterSToCServer(ctrl, &stocServer{})
	clientConn, errC := totem.Handle[SToCServer](ctrl, stream)
	ctos := NewCToSClient(clientConn)

	Expect(cf(ctos, errC)).To(Succeed())
}

func runScenarioB(sf, cf handlerFunc[CalculatorClient]) {
	server := newScenarioB(sf)
	client := server.Dial()
	stream, err := client.TestStream(context.Background())
	Expect(err).NotTo(HaveOccurred())

	ctrl := totem.NewController(stream)
	RegisterCalculatorServer(ctrl, &calculatorServer{})
	clientConn, errC := totem.Handle[CalculatorServer](ctrl, stream)
	ctos := NewCalculatorClient(clientConn)

	Expect(cf(ctos, errC)).To(Succeed())
}
