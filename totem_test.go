package totem_test

import (
	context "context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	sync "sync"
	"time"

	"github.com/kralicky/totem"
	"github.com/kralicky/totem/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

var (
	timeout = time.Second * 6
)

var _ = Describe("Test", func() {
	It("should work with two different servers", func() {
		a, b := make(chan struct{}), make(chan struct{})
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				cc, errC := ts.Serve()

				checkDecrement(cc)
				close(a)

				return <-errC
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				ts, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				decSrv := decrementServer{}
				test.RegisterDecrementServer(ts, &decSrv)
				cc, errC := ts.Serve()

				checkIncrement(cc)
				close(b)

				return <-errC
			},
		}
		tc.Run(a, b)
	})

	It("should work with the same server on both sides", func() {
		a, b := make(chan struct{}), make(chan struct{})
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				cc, errC := ts.Serve()

				checkIncrement(cc)
				close(a)

				return <-errC
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				ts, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				cc, errC := ts.Serve()

				checkIncrement(cc)
				close(b)

				return <-errC
			},
		}
		tc.Run(a, b)
	})

	It("should handle the server ending the stream", func() {
		done := make(chan struct{})
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				cc, _ := ts.Serve()

				checkIncrement(cc)

				<-done
				return nil
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				ts, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				cc, errC := ts.Serve()

				checkIncrement(cc)

				close(done)

				select {
				case err := <-errC:
					return err
				case <-time.After(timeout):
					return errors.New("timeout")
				}
			},
		}
		tc.Run()
	})

	It("should work with multiple services", func() {
		a, b := make(chan struct{}), make(chan struct{})
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				incSrv := incrementServer{}
				hashSrv := hashServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				test.RegisterHashServer(ts, &hashSrv)
				cc, errC := ts.Serve()

				checkDecrement(cc)
				close(a)

				return <-errC
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				ts, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				decSrv := decrementServer{}
				test.RegisterDecrementServer(ts, &decSrv)
				cc, errC := ts.Serve()

				checkIncrement(cc)
				checkHash(cc)
				close(b)

				return <-errC
			},
		}
		tc.Run(a, b)
	})

	It("should correctly splice streams", func() {
		s1 := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s1"))
				if err != nil {
					return err
				}
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				_, errC := ts.Serve()
				return <-errC
			},
		}
		s2 := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s2"))
				if err != nil {
					return err
				}
				decSrv := decrementServer{}
				test.RegisterDecrementServer(ts, &decSrv)
				_, errC := ts.Serve()
				return <-errC
			},
		}
		wait := make(chan struct{})
		done := make(chan struct{})
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("tc"))
				if err != nil {
					return err
				}
				hashSrv := hashServer{}
				test.RegisterHashServer(ts, &hashSrv)

				{
					s1Conn := s1.Dial()
					s1Client := test.NewTestClient(s1Conn)
					s1Stream, err := s1Client.TestStream(context.Background())
					if err != nil {
						return err
					}
					ts.Splice(s1Stream, totem.WithStreamName("s1"))
				}
				{
					s2Conn := s2.Dial()
					s2Client := test.NewTestClient(s2Conn)
					s2Stream, err := s2Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(s2Stream, totem.WithStreamName("s2"))
				}

				close(wait)
				_, errC := ts.Serve()
				select {
				case <-done:
					return nil
				case err := <-errC:
					return err
				}
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				<-wait
				defer close(done)
				ts, err := totem.NewServer(stream, totem.WithName("tc_client"))
				if err != nil {
					return err
				}
				cc, errC := ts.Serve()

				go func() {
					defer GinkgoRecover()
					err := <-errC
					Expect(err).To(Or(BeNil(), WithTransform(status.Code, Equal(codes.Canceled))))
				}()

				checkHash(cc)
				checkDecrement(cc)
				checkIncrement(cc)

				return nil
			},
		}
		go s1.RunServerOnly()
		go s2.RunServerOnly()
		tc.Run()
	})

	It("should handle nested spliced streams", func() {
		s1 := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s1"))
				if err != nil {
					return err
				}
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				_, errC := ts.Serve()
				return <-errC
			},
		}
		s2 := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s2"))
				if err != nil {
					return err
				}
				decSrv := decrementServer{}
				test.RegisterDecrementServer(ts, &decSrv)

				{
					s1Conn := s1.Dial()
					s1Client := test.NewTestClient(s1Conn)
					s1Stream, err := s1Client.TestStream(context.Background())
					if err != nil {
						return err
					}
					ts.Splice(s1Stream, totem.WithStreamName("s1"))
				}

				_, errC := ts.Serve()
				return <-errC
			},
		}
		wait := make(chan struct{})
		done := make(chan struct{})
		s3 := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s3"))
				if err != nil {
					return err
				}
				hashSrv := hashServer{}
				test.RegisterHashServer(ts, &hashSrv)

				{
					s2Conn := s2.Dial()
					s2Client := test.NewTestClient(s2Conn)
					s2Stream, err := s2Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(s2Stream, totem.WithStreamName("s2"))
				}

				close(wait)
				_, errC := ts.Serve()
				select {
				case <-done:
					return nil
				case err := <-errC:
					return err
				}
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				<-wait
				defer close(done)
				ts, err := totem.NewServer(stream, totem.WithName("s3_client"))
				if err != nil {
					return err
				}
				cc, _ := ts.Serve()

				checkHash(cc)
				checkDecrement(cc)
				checkIncrement(cc)

				return nil
			},
		}

		go s1.RunServerOnly()
		go s2.RunServerOnly()
		s3.Run()
	})

	It("should handle bidirectional nested spliced streams", func() {
		var s1, s2, s3, s4 *testCase

		s1 = &testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s1"))
				if err != nil {
					return err
				}
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				_, errC := ts.Serve()
				return <-errC
			},
		}

		s2 = &testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s2"))
				if err != nil {
					return err
				}
				decSrv := decrementServer{}
				test.RegisterDecrementServer(ts, &decSrv)

				{
					s1Conn := s1.Dial()
					s1Client := test.NewTestClient(s1Conn)
					s1Stream, err := s1Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(s1Stream, totem.WithStreamName("s1"))
				}

				_, errC := ts.Serve()

				return <-errC
			},
		}

		s3 = &testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s3"))
				if err != nil {
					return err
				}
				hashSrv := hashServer{}
				test.RegisterHashServer(ts, &hashSrv)

				{
					s4Conn := s4.Dial()
					s4Client := test.NewTestClient(s4Conn)
					s4Stream, err := s4Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(s4Stream, totem.WithStreamName("s4"))
				}

				{
					s2Conn := s2.Dial()
					s2Client := test.NewTestClient(s2Conn)
					s2Stream, err := s2Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(s2Stream, totem.WithStreamName("s2"))
				}

				_, errC := ts.Serve()

				return <-errC
			},
		}

		s4 = &testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s4"))
				if err != nil {
					return err
				}
				mulSrv := multiplyServer{}
				test.RegisterMultiplyServer(ts, &mulSrv)

				_, errC := ts.Serve()

				return <-errC
			},
		}

		s1.AsyncRunServerOnly()
		s4.AsyncRunServerOnly()
		s2.AsyncRunServerOnly()
		s3.AsyncRunServerOnly()

		{
			s3Conn := s3.Dial()
			s3Client := test.NewTestClient(s3Conn)
			s3Stream, err := s3Client.TestStream(context.Background())
			Expect(err).NotTo(HaveOccurred())
			ts, err := totem.NewServer(s3Stream, totem.WithName("s3_client"))
			Expect(err).NotTo(HaveOccurred())
			cc, _ := ts.Serve()

			checkIncrement(cc)
			checkDecrement(cc)
			checkMultiply(cc)
			checkHash(cc)
		}
	})
	It("should handle QOS options", func() {
		const numMsgs = 1e3
		s1c := make(chan string, numMsgs)
		s2c := make(chan string, numMsgs)
		s1 := &testCase{
			ServerOptions: defaultServerOpts,
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s1"))
				if err != nil {
					return err
				}
				ns := notifyServer{
					C: s1c,
				}
				test.RegisterNotifyServer(ts, &ns)
				_, errC := ts.Serve()
				return <-errC
			},
		}
		s2 := &testCase{
			ServerOptions: defaultServerOpts,
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s2"))
				if err != nil {
					return err
				}
				ns := notifyServer{
					C: s2c,
				}
				test.RegisterNotifyServer(ts, &ns)
				_, errC := ts.Serve()
				return <-errC
			},
		}
		s1.AsyncRunServerOnly()
		s2.AsyncRunServerOnly()

		s3 := &testCase{
			ServerOptions: defaultServerOpts,
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s3"))
				if err != nil {
					return err
				}

				{
					s1Conn := s1.Dial()
					s1Client := test.NewTestClient(s1Conn)
					s1Stream, err := s1Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(s1Stream, totem.WithStreamName("s1"))
				}
				{
					s2Conn := s2.Dial()
					s2Client := test.NewTestClient(s2Conn)
					s2Stream, err := s2Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(s2Stream, totem.WithStreamName("s2"))
				}

				_, errC := ts.Serve()

				return <-errC
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				tc, err := totem.NewServer(stream, totem.WithName("s3_client"))
				if err != nil {
					return err
				}
				cc, _ := tc.Serve()
				notifyClient := test.NewNotifyClient(cc)
				for i := 0; i < numMsgs; i++ {
					notifyClient.Notify(context.Background(), &test.String{Str: fmt.Sprintf("msg%d", i)})
				}
				return nil
			},
		}

		done := make(chan struct{})
		go s3.Run(done)

		Eventually(s1c).Should(HaveLen(numMsgs))
		Eventually(s2c).Should(HaveLen(numMsgs))
		close(done)
		for i := 0; i < numMsgs; i++ {
			Expect(<-s1c).To(Equal(fmt.Sprintf("msg%d", i)))
			Expect(<-s2c).To(Equal(fmt.Sprintf("msg%d", i)))
		}
		Consistently(s1c).Should(BeEmpty())
		Consistently(s2c).Should(BeEmpty())
	})
	It("should ensure broadcast messages are reentrant", func() {
		var a1, a2, s1 *testCase

		/*
			A1──┐
					│
					├───C1────S1
					│
			A2──┘

		*/
		a1 = &testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("a1"))
				if err != nil {
					return err
				}
				test.RegisterSleepServer(ts, &sleepServer{
					Name: "a1",
				})
				_, errC := ts.Serve()
				return <-errC
			},
		}

		a2 = &testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("a2"))
				if err != nil {
					return err
				}
				test.RegisterSleepServer(ts, &sleepServer{
					Name: "a2",
				})
				_, errC := ts.Serve()
				return <-errC
			},
		}

		// done := make(chan struct{})
		s1 = &testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream, totem.WithName("s1"))
				if err != nil {
					return err
				}
				{
					a1Conn := a1.Dial()
					a1Client := test.NewTestClient(a1Conn)
					a1Stream, err := a1Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(a1Stream, totem.WithStreamName("a1"))
				}

				{
					a2Conn := a2.Dial()
					a2Client := test.NewTestClient(a2Conn)
					a2Stream, err := a2Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(a2Stream, totem.WithStreamName("a2"))
				}

				_, errC := ts.Serve()
				return <-errC
			},
		}

		a1.AsyncRunServerOnly()
		a2.AsyncRunServerOnly()
		s1.AsyncRunServerOnly()

		var sleepClient1, sleepClient2 test.SleepClient
		{
			s1Conn := s1.Dial()
			s1Client := test.NewTestClient(s1Conn)
			s1Stream, err := s1Client.TestStream(context.Background())
			Expect(err).NotTo(HaveOccurred())
			ts, err := totem.NewServer(s1Stream, totem.WithName("s1_client_1"))
			Expect(err).NotTo(HaveOccurred())
			cc, _ := ts.Serve()

			sleepClient1 = test.NewSleepClient(cc)
		}

		{
			s1Conn := s1.Dial()
			s1Client := test.NewTestClient(s1Conn)
			s1Stream, err := s1Client.TestStream(context.Background())
			Expect(err).NotTo(HaveOccurred())
			ts, err := totem.NewServer(s1Stream, totem.WithName("s1_client_2"))
			Expect(err).NotTo(HaveOccurred())
			cc, _ := ts.Serve()

			sleepClient2 = test.NewSleepClient(cc)
		}

		time.Sleep(500 * time.Millisecond)
		var wg sync.WaitGroup
		wg.Add(2)
		fmt.Println("starting")
		ctx, span := totem.Tracer().Start(context.Background(), "sleep-test")
		go func() {
			defer wg.Done()
			sleepClient1.Sleep(ctx, &test.SleepRequest{
				Duration: durationpb.New(1 * time.Second),
				Filter:   "a1",
			})
		}()
		time.Sleep(500 * time.Millisecond)
		span.AddEvent("2nd sleep")
		go func() {
			defer wg.Done()
			sleepClient2.Sleep(ctx, &test.SleepRequest{
				Duration: durationpb.New(1 * time.Second),
				Filter:   "a2",
			})
		}()

		wg.Wait()
		span.End()
	})
})

func checkIncrement(cc grpc.ClientConnInterface) {
	incClient := test.NewIncrementClient(cc)
	result, err := incClient.Inc(context.Background(), &test.Number{
		Value: 0,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Value).To(BeEquivalentTo(1))
}

func checkDecrement(cc grpc.ClientConnInterface) {
	decClient := test.NewDecrementClient(cc)
	result, err := decClient.Dec(context.Background(), &test.Number{
		Value: 0,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Value).To(BeEquivalentTo(-1))
}

func checkMultiply(cc grpc.ClientConnInterface) {
	mulClient := test.NewMultiplyClient(cc)
	result, err := mulClient.Mul(context.Background(), &test.Operands{
		A: 2,
		B: 3,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result.Value).To(BeEquivalentTo(6))
}

func checkHash(cc grpc.ClientConnInterface) {
	hashClient := test.NewHashClient(cc)
	result, err := hashClient.Hash(context.Background(), &test.String{
		Str: "hello",
	})
	Expect(err).NotTo(HaveOccurred())
	expectedHash := sha1.Sum([]byte("hello"))
	Expect(result.Str).To(Equal(hex.EncodeToString(expectedHash[:])))
}
