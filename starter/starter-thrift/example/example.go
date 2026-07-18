/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"go-spring.org/starter-thrift/example/idl/proto"

	"github.com/apache/thrift/lib/go/thrift"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-thrift"
)

func init() {
	// Register the Echo controller and the (wrapped) thrift.TProcessor bean.
	// The starter picks up any bean that satisfies thrift.TProcessor and
	// hands it to TSimpleServer, so wrapping the generated processor is
	// enough to insert middleware — no starter changes required.
	gs.Provide(&Controller{})
	gs.Provide(func(c *Controller) thrift.TProcessor {
		return newLoggingProcessor(proto.NewEchoServiceProcessor(c))
	})
}

// Controller is the EchoService handler. Echo returns the request message
// unchanged, giving the client a deterministic value to assert on.
type Controller struct{}

// Echo satisfies proto.EchoService: it echoes the incoming message back.
func (c *Controller) Echo(ctx context.Context, req *proto.EchoRequest) (*proto.EchoResponse, error) {
	return &proto.EchoResponse{Message: req.Message}, nil
}

// middlewareCallCount records how many times loggingProcessor observed a
// Thrift RPC. Because the example runs the server and the test client in
// one process, the client can read this counter to prove the decorator
// was actually invoked per call.
var middlewareCallCount int64

// loggingProcessor is a thrift.TProcessor decorator that logs each RPC's
// method name and then dispatches to the wrapped (generated) processor's
// per-method handler. It is the Thrift equivalent of a gRPC
// UnaryServerInterceptor or an HTTP middleware.
//
// Note on structure: a naive "read header, log, then call inner.Process"
// pattern is not workable in Thrift Go because inner.Process would read a
// fresh message header from the wire (iprot state is single-shot). The
// idiomatic wrapping therefore reproduces the tiny dispatch loop that the
// generated processor performs, but layered on top of the inner
// processor's ProcessorMap — the outer type still satisfies
// thrift.TProcessor and remains a drop-in decorator.
type loggingProcessor struct {
	inner thrift.TProcessor
}

func newLoggingProcessor(inner thrift.TProcessor) *loggingProcessor {
	return &loggingProcessor{inner: inner}
}

// Process implements thrift.TProcessor. It logs the method name, then
// delegates to the inner processor's registered TProcessorFunction for
// that method — the same table that inner.Process would consult.
func (p *loggingProcessor) Process(ctx context.Context, iprot, oprot thrift.TProtocol) (bool, thrift.TException) {
	name, _, seqId, err := iprot.ReadMessageBegin(ctx)
	if err != nil {
		return false, thrift.WrapTException(err)
	}

	n := atomic.AddInt64(&middlewareCallCount, 1)
	log.Infof(ctx, log.TagAppDef,
		"thrift middleware: method=%s seq=%d count=%d", name, seqId, n)

	if processor, ok := p.inner.ProcessorMap()[name]; ok {
		return processor.Process(ctx, seqId, iprot, oprot)
	}

	// Unknown method: mirror the standard generated behaviour so the
	// wire protocol stays well-formed.
	_ = iprot.Skip(ctx, thrift.STRUCT)
	_ = iprot.ReadMessageEnd(ctx)
	x := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "Unknown function "+name)
	_ = oprot.WriteMessageBegin(ctx, name, thrift.EXCEPTION, seqId)
	_ = x.Write(ctx, oprot)
	_ = oprot.WriteMessageEnd(ctx)
	_ = oprot.Flush(ctx)
	return false, x
}

// ProcessorMap forwards to the wrapped processor's dispatch table.
func (p *loggingProcessor) ProcessorMap() map[string]thrift.TProcessorFunction {
	return p.inner.ProcessorMap()
}

// AddToProcessorMap forwards new registrations to the inner processor.
func (p *loggingProcessor) AddToProcessorMap(name string, tpf thrift.TProcessorFunction) {
	p.inner.AddToProcessorMap(name, tpf)
}

func main() {
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest()
	}()

	// HTTP server is disabled via conf/app.properties. Run the app as-is
	// so the example matches the production wiring: no inline overrides.
	gs.Run()
}

// runTest exercises the three core Thrift features end-to-end and asserts
// on the result. On any assertion failure it exits(1); on success it sends
// SIGTERM to itself so gs.Run() shuts down cleanly.
//
// The server is configured (conf/app.properties) with compact protocol +
// framed transport, so the client MUST match: it wraps the raw TSocket in
// TFramedTransport and uses TCompactProtocol. A protocol or transport
// mismatch between client and server would deadlock or corrupt reads.
func runTest() {
	ctx := context.Background()

	socket := thrift.NewTSocketConf(":9292", nil)
	// Framed transport must match the server's framed transport factory.
	transport := thrift.NewTFramedTransportConf(socket, nil)
	defer transport.Close()

	// Compact protocol must match the server's compact protocol factory.
	protocolFactory := thrift.NewTCompactProtocolFactoryConf(nil)
	client := proto.NewEchoServiceClientFactory(transport, protocolFactory)

	if err := transport.Open(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error opening transport: %v", err)
		os.Exit(1)
	}

	// Feature 1: Echo RPC — canonical request/response.
	resp1, err := client.Echo(ctx, &proto.EchoRequest{Message: "Hello, Thrift!"})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error calling Echo (1): %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", resp1.Message)
	if resp1.Message != "Hello, Thrift!" {
		log.Errorf(ctx, log.TagAppDef, "unexpected echo body (1): %q", resp1.Message)
		os.Exit(1)
	}

	// Feature 3: A second round-trip with a distinct payload. This proves
	// the wrapped processor forwards correctly across independent calls,
	// and — combined with the counter check below — that the middleware
	// runs per RPC rather than once at startup. (Server uses framed +
	// compact; the client above matches, see runTest doc.)
	resp2, err := client.Echo(ctx, &proto.EchoRequest{Message: "Middleware works!"})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "Error calling Echo (2): %v", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", resp2.Message)
	if resp2.Message != "Middleware works!" {
		log.Errorf(ctx, log.TagAppDef, "unexpected echo body (2): %q", resp2.Message)
		os.Exit(1)
	}

	// Feature 2: TProcessor middleware/decorator invocation count.
	if got := atomic.LoadInt64(&middlewareCallCount); got != 2 {
		log.Errorf(ctx, log.TagAppDef,
			"loggingProcessor middleware fired %d time(s), want 2", got)
		os.Exit(1)
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
