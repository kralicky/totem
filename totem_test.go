package totem_test

import (
	context "context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/kralicky/totem"
	"github.com/kralicky/totem/test"
	. "github.com/onsi/ginkgo/v2"
)

var (
	timeout = time.Second * 10
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
				if err != nil {
					return err
				}
				if result.Value != 1 {
					return fmt.Errorf("expected 1, got %d", result.Value)
				}

				select {
				case <-done:
				case <-time.After(timeout):
					return errors.New("timeout")
				}
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
				if err != nil {
					return err
				}
				if result.Value != 3 {
					return fmt.Errorf("expected 3, got %d", result.Value)
				}

				select {
				case <-done:
				case <-time.After(timeout):
					return errors.New("timeout")
				}
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
				if err != nil {
					return err
				}
				if result.Value != 11 {
					return fmt.Errorf("expected 11, got %d", result.Value)
				}

				select {
				case <-done:
				case <-time.After(timeout):
					return errors.New("timeout")
				}
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
				if err != nil {
					return err
				}
				if result.Value != 6 {
					return fmt.Errorf("expected 6, got %d", result.Value)
				}

				select {
				case <-done:
				case <-time.After(timeout):
					return errors.New("timeout")
				}
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
				if err != nil {
					return err
				}
				if result.Value != 11 {
					return fmt.Errorf("expected 11, got %d", result.Value)
				}

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
				if err != nil {
					return err
				}
				if result.Value != 6 {
					return fmt.Errorf("expected 6, got %d", result.Value)
				}

				select {
				case err := <-errC:
					if !errors.Is(err, io.EOF) {
						return fmt.Errorf("expected EOF, got %v", err)
					}
				case <-time.After(timeout):
					return errors.New("timeout")
				}
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
				if err != nil {
					return err
				}
				if result.Value != -1 {
					return fmt.Errorf("expected -1, got %d", result.Value)
				}

				select {
				case <-done:
				case <-time.After(timeout):
					return errors.New("timeout")
				}
				select {
				case <-done2:
				case <-time.After(timeout):
					return errors.New("timeout")
				}
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
				if err != nil {
					return err
				}
				if result.Value != -4 {
					return fmt.Errorf("expected -4, got %d", result.Value)
				}

				hashClient := test.NewHashClient(cc)
				result2, err := hashClient.Hash(context.Background(), &test.String{
					Str: "hello",
				})
				if err != nil {
					return err
				}
				expectedHash := sha1.Sum([]byte("hello"))
				if result2.Str != hex.EncodeToString(expectedHash[:]) {
					return fmt.Errorf("expected %s, got %s", hex.EncodeToString(expectedHash[:]), result2.Str)
				}

				select {
				case <-done:
				case <-time.After(timeout):
					return errors.New("timeout")
				}
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
				test.RegisterIncrementServer(ts, &incSrv)
				_, errC := ts.Serve()
				return <-errC
			},
		}
		s2 := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts := totem.NewServer(stream)
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
				defer GinkgoRecover()
				ts := totem.NewServer(stream)
				hashSrv := hashServer{}
				test.RegisterHashServer(ts, &hashSrv)

				{
					incSvcDesc, err := totem.LoadServiceDesc(&test.Increment_ServiceDesc)
					if err != nil {
						return err
					}
					s1Conn := s1.Dial()
					s1Client := test.NewTestClient(s1Conn)
					s1Stream, err := s1Client.TestStream(context.Background())
					if err != nil {
						return err
					}
					ts.Splice(s1Stream, incSvcDesc)
				}
				{
					decSvcDesc, err := totem.LoadServiceDesc(&test.Decrement_ServiceDesc)
					if err != nil {
						return err
					}
					s2Conn := s2.Dial()
					s2Client := test.NewTestClient(s2Conn)
					s2Stream, err := s2Client.TestStream(context.Background())
					if err != nil {
						return err
					}

					ts.Splice(s2Stream, decSvcDesc)
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
				ts := totem.NewServer(stream)
				cc, errC := ts.Serve()

				go func() {
					ok := <-errC
					if ok != nil && ok != io.EOF {
						panic(ok)
					}
					if err := <-errC; err != nil {
						panic(err)
					}
				}()

				hashClient := test.NewHashClient(cc)
				incClient := test.NewIncrementClient(cc)
				decClient := test.NewDecrementClient(cc)

				{
					result, err := hashClient.Hash(context.Background(), &test.String{
						Str: "hello",
					})
					if err != nil {
						return err
					}
					expectedHash := sha1.Sum([]byte("hello"))
					if result.Str != hex.EncodeToString(expectedHash[:]) {
						return fmt.Errorf("unexpected hash result: %s", result.Str)
					}
				}
				{
					result, err := decClient.Dec(context.Background(), &test.Number{
						Value: 0,
					})
					if err != nil {
						return err
					}
					if result.Value != -1 {
						return fmt.Errorf("expected -1, got %d", result.Value)
					}
				}
				{
					result, err := incClient.Inc(context.Background(), &test.Number{
						Value: 5,
					})
					if err != nil {
						return err
					}
					if result.Value != 6 {
						return fmt.Errorf("expected 6, got %d", result.Value)
					}
				}

				close(done)
				return nil
			},
		}
		go s1.RunServerOnly()
		go s2.RunServerOnly()
		tc.Run()
	})
})
