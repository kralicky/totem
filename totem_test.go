package totem_test

import (
	context "context"
	"crypto/sha1"
	"encoding/hex"
	"io"

	"github.com/kralicky/totem"
	. "github.com/kralicky/totem/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test", func() {
	It("should work with two different servers", func() {
		tc := testCase{
			ServerHandler: func(stream Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				done := incSrv.LimitRequests(1)
				RegisterIncrementServer(ts, &incSrv)
				cc, _ := ts.Serve()

				decClient := NewDecrementClient(cc)
				result, err := decClient.Dec(context.Background(), &Number{
					Value: 2,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(1)))

				Eventually(done).Should(BeClosed())
				return nil
			},
			ClientHandler: func(stream Test_TestStreamClient) error {
				ts := totem.NewServer(stream)
				decSrv := decrementServer{}
				done := decSrv.LimitRequests(1)
				RegisterDecrementServer(ts, &decSrv)
				cc, _ := ts.Serve()

				incClient := NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &Number{
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
			ServerHandler: func(stream Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				done := incSrv.LimitRequests(1)
				RegisterIncrementServer(ts, &incSrv)
				cc, _ := ts.Serve()

				incClient := NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &Number{
					Value: 10,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(11)))

				Eventually(done).Should(BeClosed())
				return nil
			},
			ClientHandler: func(stream Test_TestStreamClient) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				done := incSrv.LimitRequests(1)
				RegisterIncrementServer(ts, &incSrv)
				cc, _ := ts.Serve()

				incClient := NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &Number{
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
			ServerHandler: func(stream Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				RegisterIncrementServer(ts, &incSrv)
				cc, _ := ts.Serve()

				incClient := NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &Number{
					Value: 10,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(11)))

				return nil
			},
			ClientHandler: func(stream Test_TestStreamClient) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				RegisterIncrementServer(ts, &incSrv)
				cc, errC := ts.Serve()

				incClient := NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &Number{
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
			ServerHandler: func(stream Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
				incSrv := incrementServer{}
				done := incSrv.LimitRequests(1)
				hashSrv := hashServer{}
				done2 := hashSrv.LimitRequests(1)
				RegisterIncrementServer(ts, &incSrv)
				RegisterHashServer(ts, &hashSrv)
				cc, _ := ts.Serve()

				decClient := NewDecrementClient(cc)
				result, err := decClient.Dec(context.Background(), &Number{
					Value: 0,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(-1)))

				Eventually(done).Should(BeClosed())
				Eventually(done2).Should(BeClosed())
				return nil
			},
			ClientHandler: func(stream Test_TestStreamClient) error {
				ts := totem.NewServer(stream)
				decSrv := decrementServer{}
				done := decSrv.LimitRequests(1)
				RegisterDecrementServer(ts, &decSrv)
				cc, _ := ts.Serve()

				incClient := NewIncrementClient(cc)
				result, err := incClient.Inc(context.Background(), &Number{
					Value: -5,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Value).To(Equal(int64(-4)))

				hashClient := NewHashClient(cc)
				result2, err := hashClient.Hash(context.Background(), &String{
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
})
