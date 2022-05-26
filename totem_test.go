package totem_test

import (
	context "context"
	"crypto/sha1"
	"encoding/hex"
	"io"

	"github.com/kralicky/totem"
	"github.com/kralicky/totem/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test", func() {
	It("should work with two different servers", func() {
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				done := incSrv.LimitRequests(1)
				test.RegisterIncrementServer(ts, &incSrv)
				cc, _ := ts.Serve()

				decClient := test.NewDecrementClient(cc)
				result, err := decClient.Dec(context.Background(), &test.Number{
					Value: 2,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(1)))

				Eventually(done).Should(BeClosed())
				return nil
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				ts := totem.NewServer(stream)
				decSrv := decrementServer{}
				done := decSrv.LimitRequests(1)
				test.RegisterDecrementServer(ts, &decSrv)
				cc, _ := ts.Serve()

				incClient := test.NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &test.Number{
					Value: 2,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(3)))

				Eventually(done).Should(BeClosed())
				return nil
			},
		}
		tc.Run()
	})

	It("should work with the same server on both sides", func() {
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				done := incSrv.LimitRequests(1)
				test.RegisterIncrementServer(ts, &incSrv)
				cc, _ := ts.Serve()

				incClient := test.NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &test.Number{
					Value: 10,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(11)))

				Eventually(done).Should(BeClosed())
				return nil
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				done := incSrv.LimitRequests(1)
				test.RegisterIncrementServer(ts, &incSrv)
				cc, _ := ts.Serve()

				incClient := test.NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &test.Number{
					Value: 5,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(6)))

				Eventually(done).Should(BeClosed())
				return nil
			},
		}
		tc.Run()
	})

	It("should handle the server ending the stream", func() {
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				cc, _ := ts.Serve()

				incClient := test.NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &test.Number{
					Value: 10,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(11)))

				return nil
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				test.RegisterIncrementServer(ts, &incSrv)
				cc, errC := ts.Serve()

				incClient := test.NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &test.Number{
					Value: 5,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(6)))

				Eventually(errC).Should(Receive(MatchError(io.EOF)))
				return nil
			},
		}
		tc.Run()
	})

	It("should work with multiple services", func() {
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				done := incSrv.LimitRequests(1)
				hashSrv := hashServer{}
				done2 := hashSrv.LimitRequests(1)
				test.RegisterIncrementServer(ts, &incSrv)
				test.RegisterHashServer(ts, &hashSrv)
				cc, _ := ts.Serve()

				decClient := test.NewDecrementClient(cc)
				result, err := decClient.Dec(context.Background(), &test.Number{
					Value: 0,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(-1)))

				Eventually(done).Should(BeClosed())
				Eventually(done2).Should(BeClosed())
				return nil
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				ts := totem.NewServer(stream)
				decSrv := decrementServer{}
				done := decSrv.LimitRequests(1)
				test.RegisterDecrementServer(ts, &decSrv)
				cc, _ := ts.Serve()

				incClient := test.NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &test.Number{
					Value: -5,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(-4)))

				hashClient := test.NewHashClient(cc)
				result2, err := hashClient.Hash(context.Background(), &test.String{
					Str: "hello",
				})
				Expect(err).NotTo(HaveOccurred())
				expectedHash := sha1.Sum([]byte("hello"))
				Expect(result2.Str).To(Equal(hex.EncodeToString(expectedHash[:])))

				Eventually(done).Should(BeClosed())
				return nil
			},
		}
		tc.Run()
	})

	It("should correctly splice streams", func() {
		s1 := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				done := incSrv.LimitRequests(1)
				test.RegisterIncrementServer(ts, &incSrv)
				ts.Serve()
				Eventually(done).Should(BeClosed())
				return nil
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				return nil
			},
		}
		s2 := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				decSrv := decrementServer{}
				done := decSrv.LimitRequests(1)
				test.RegisterDecrementServer(ts, &decSrv)
				ts.Serve()
				Eventually(done).Should(BeClosed())
				return nil
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				return nil
			},
		}
		tc := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				hashSrv := hashServer{}
				done := hashSrv.LimitRequests(1)
				test.RegisterHashServer(ts, &hashSrv)

				{
					incSvcDesc, err := totem.LoadServiceDesc(&test.Increment_ServiceDesc)
					Expect(err).NotTo(HaveOccurred())

					s1Conn := s1.Dial()
					s1Client := test.NewTestClient(s1Conn)
					s1Stream, err := s1Client.TestStream(context.Background())
					Expect(err).NotTo(HaveOccurred())

					ts.Splice(s1Stream, incSvcDesc)
				}
				{
					decSvcDesc, err := totem.LoadServiceDesc(&test.Decrement_ServiceDesc)
					Expect(err).NotTo(HaveOccurred())

					s2Conn := s2.Dial()
					s2Client := test.NewTestClient(s2Conn)
					s2Stream, err := s2Client.TestStream(context.Background())
					Expect(err).NotTo(HaveOccurred())

					ts.Splice(s2Stream, decSvcDesc)
				}

				ts.Serve()
				Eventually(done).Should(BeClosed())
				return nil
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				ts := totem.NewServer(stream)
				cc, _ := ts.Serve()
				hashClient := test.NewHashClient(cc)
				incClient := test.NewIncrementClient(cc)
				decClient := test.NewDecrementClient(cc)

				{
					result, err := hashClient.Hash(context.Background(), &test.String{
						Str: "hello",
					})
					Expect(err).NotTo(HaveOccurred())
					expectedHash := sha1.Sum([]byte("hello"))
					Expect(result.Str).To(Equal(hex.EncodeToString(expectedHash[:])))
				}
				{
					result, err := decClient.Dec(context.Background(), &test.Number{
						Value: 0,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Value).To(Equal(int64(-1)))
				}
				{
					result, err := incClient.Inc(context.Background(), &test.Number{
						Value: 5,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Value).To(Equal(int64(6)))
				}

				return nil
			},
		}
		go s1.Run()
		go s2.Run()
		tc.Run()
	})
})
