package totem_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	sync "sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure/table"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"

	"github.com/kralicky/totem"
	"github.com/kralicky/totem/test"
)

type benchmarkParams struct {
	payloadSize     uint64
	parallelClients uint64
	serverOptions   []grpc.ServerOption
	clientOptions   []grpc.DialOption
}

var sizeOptions = map[string]func(*benchmarkParams){
	"64b": func(p *benchmarkParams) {
		p.payloadSize = 64
	},
	"128b": func(p *benchmarkParams) {
		p.payloadSize = 128
	},
	"1kb": func(p *benchmarkParams) {
		p.payloadSize = 1024
	},
	"10kb": func(p *benchmarkParams) {
		p.payloadSize = 1024 * 10
	},
	"100kb": func(p *benchmarkParams) {
		p.payloadSize = 1024 * 100
	},
	"1mb": func(p *benchmarkParams) {
		p.payloadSize = 1024 * 1024
	},
}

var parallelOptions = map[string]func(*benchmarkParams){
	"1client": func(p *benchmarkParams) {
		p.parallelClients = 1
	},
	// "10clients": func(p *benchmarkParams) {
	// 	p.parallelClients = 10
	// },
	"100clients": func(p *benchmarkParams) {
		p.parallelClients = 100
	},
	"1000clients": func(p *benchmarkParams) {
		p.parallelClients = 1000
	},
}

var grpcOptions = map[string]func(*benchmarkParams){
	"default": func(p *benchmarkParams) {},
	// "unbuffered": func(p *benchmarkParams) {
	// 	p.serverOptions = []grpc.ServerOption{grpc.WriteBufferSize(0), grpc.ReadBufferSize(0)}
	// 	p.clientOptions = []grpc.DialOption{grpc.WithWriteBufferSize(0), grpc.WithReadBufferSize(0)}
	// },
	// "2xbuffer": func(p *benchmarkParams) {
	// 	p.serverOptions = []grpc.ServerOption{grpc.WriteBufferSize(64 * 1024), grpc.ReadBufferSize(64 * 1024)}
	// 	p.clientOptions = []grpc.DialOption{grpc.WithWriteBufferSize(64 * 1024), grpc.WithReadBufferSize(64 * 1024)}
	// },

	// "2xwindow": func(p *benchmarkParams) {
	// 	p.serverOptions = []grpc.ServerOption{grpc.InitialWindowSize(128 * 1024), grpc.InitialConnWindowSize(128 * 1024)}
	// 	p.clientOptions = []grpc.DialOption{grpc.WithInitialWindowSize(128 * 1024), grpc.WithInitialConnWindowSize(128 * 1024)}
	// },
	// "4xbuffer": func(p *benchmarkParams) {
	// 	p.serverOptions = []grpc.ServerOption{grpc.WriteBufferSize(128 * 1024), grpc.ReadBufferSize(128 * 1024)}
	// 	p.clientOptions = []grpc.DialOption{grpc.WithWriteBufferSize(128 * 1024), grpc.WithReadBufferSize(128 * 1024)}
	// },
}

var params = make(map[string]benchmarkParams)

func init() {
	for sizeName, sizeFunc := range sizeOptions {
		for parallelName, parallelFunc := range parallelOptions {
			for grpcName, grpcFunc := range grpcOptions {
				name := sizeName + "-" + parallelName + "-" + grpcName
				bp := benchmarkParams{}
				sizeFunc(&bp)
				parallelFunc(&bp)
				grpcFunc(&bp)
				params[name] = bp
			}
		}
	}
}

var _ = Describe("Stream", Ordered, func() {
	var unaryClient, totemClient test.EchoClient
	var doTeardown func()
	doSetup := func(bp benchmarkParams) {
		// unary server
		listener, err := net.Listen("tcp4", ":0")
		Expect(err).NotTo(HaveOccurred())

		server := grpc.NewServer(bp.serverOptions...)
		test.RegisterEchoServer(server, &echoServer{})
		go server.Serve(listener)
		conn, err := grpc.DialContext(context.Background(), listener.Addr().String(),
			append(bp.clientOptions,
				grpc.WithInsecure(),
				grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
			)...)
		Expect(err).NotTo(HaveOccurred())
		unaryClient = test.NewEchoClient(conn)

		// totem server
		clientReady := make(chan struct{})
		done := make(chan struct{})
		tc1 := testCase{
			ServerHandler: func(stream test.Test_TestStreamServer) error {
				ts, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				echoSrv := &echoServer{}
				test.RegisterEchoServer(ts, echoSrv)
				_, errC := ts.Serve()

				select {
				case <-done:
					return nil
				case err := <-errC:
					return err
				}
			},
			ClientHandler: func(stream test.Test_TestStreamClient) error {
				tc, err := totem.NewServer(stream)
				if err != nil {
					return err
				}
				cc, errC := tc.Serve()

				totemClient = test.NewEchoClient(cc)
				close(clientReady)
				select {
				case <-done:
					return nil
				case err := <-errC:
					return err
				}
			},
			ServerOptions: bp.serverOptions,
			ClientOptions: bp.clientOptions,
		}

		go tc1.Run()

		<-clientReady

		doTeardown = func() {
			close(done)
			conn.Close()
			server.Stop()
		}
	}

	Specify("benchmarking", func() {
		tab := table.NewTable()
		tab.AppendRow(table.R(
			table.C("Name"), table.C("msgs/s"), table.C("Mbps"),
			table.Divider("="),
			"{{bold}}",
		))
		sample := func(name string, client test.EchoClient, data []byte, parallel int) func() {
			var count atomic.Int32
			var done atomic.Bool
			var wg sync.WaitGroup
			for i := 0; i < parallel; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for !done.Load() {
						totemClient.Echo(context.Background(), &test.Bytes{Data: data})
						count.Add(1)
					}
				}()
			}
			time.Sleep(1 * time.Second)
			done.Store(true)
			wg.Wait()
			return func() {
				bytesPerSecond := float64(count.Load()) * float64(len(data))
				megabitsPerSecond := bytesPerSecond * 8 / 1024 / 1024
				row := table.R()
				row.AppendCell(table.C(name), table.C(fmt.Sprint(count.Load())), table.C(fmt.Sprintf("%.1f", megabitsPerSecond)))
				tab.AppendRow(row)
			}
		}

		for name, p := range params {
			doSetup(p)
			defer doTeardown()
			name, p := name, p
			data := make([]byte, p.payloadSize)
			rand.Read(data)
			var wg sync.WaitGroup
			var recordTotemResults, recordUnaryResults func()
			wg.Add(1)
			go func() {
				defer wg.Done()
				recordTotemResults = sample("totem: "+name, totemClient, append([]byte(nil), data...), int(p.parallelClients))
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				recordUnaryResults = sample("unary: "+name, unaryClient, append([]byte(nil), data...), int(p.parallelClients))
			}()
			wg.Wait()
			if recordTotemResults != nil {
				recordTotemResults()
			}
			if recordUnaryResults != nil {
				recordUnaryResults()
			}
		}

		AddReportEntry("benchmarking results", tab.Render())
	})
})
