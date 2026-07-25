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

// Package main is the observability smoke test for starter-grpc. It proves:
//   1. Trace context propagation via gRPC metadata (W3C traceparent).
//   2. Metrics emission (rpc.server.request_count etc. on actuator /metrics).
//   3. Trace-log correlation (trace_id/span_id in log fields).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-actuator"
	_ "go-spring.org/starter-otel"

	StarterGrpc "go-spring.org/starter-grpc"
	"go-spring.org/starter-grpc/example/idl/proto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var tag = log.RegisterBizTag("example-otel", "")

func init() {
	gs.Provide(&Controller{})
	gs.Provide(func(c *Controller) StarterGrpc.ServiceRegister {
		return func(svr *grpc.Server) {
			proto.RegisterEchoServiceServer(svr, c)
		}
	})
}

type Controller struct {
	proto.UnimplementedEchoServiceServer
}

// Echo returns the request message and includes correlation fields in the
// response. The tracing interceptor (enabled via config) extracts the W3C
// trace context from gRPC metadata before this handler runs.
func (c *Controller) Echo(ctx context.Context, req *proto.EchoRequest) (*proto.EchoResponse, error) {
	log.Infof(ctx, tag, "handling Echo: %s", req.Message)

	fields := log.FieldsFromContext(ctx)
	traceID, spanID := fieldValue(fields, "trace_id"), fieldValue(fields, "span_id")

	// Read the inbound traceparent from gRPC metadata to echo back.
	md, _ := metadata.FromIncomingContext(ctx)
	traceParent := ""
	if v := md.Get("traceparent"); len(v) > 0 {
		traceParent = v[0]
	}

	return &proto.EchoResponse{Message: fmt.Sprintf("%s|%s|%s|%s", req.Message, traceParent, traceID, spanID)}, nil
}

func fieldValue(fields []log.Field, key string) string {
	for _, f := range fields {
		if f.Key == key {
			if s, ok := f.Any.(string); ok {
				return s
			}
		}
	}
	return ""
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	if !*manual {
		go func() {
			time.Sleep(700 * time.Millisecond)
			runTest()
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

func runTest() {
	bgCtx := context.Background()
	ctx, parentSpan := otel.Tracer("example-otel").Start(bgCtx, "grpc-e2e-test")
	parentTraceID := parentSpan.SpanContext().TraceID().String()

	conn, err := grpc.NewClient(":9494", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect:", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Inject W3C trace context into gRPC outgoing metadata.
	md := metadata.New(nil)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(md))
	gctx := metadata.NewOutgoingContext(ctx, md)

	client := proto.NewEchoServiceClient(conn)
	response, err := client.Echo(gctx, &proto.EchoRequest{Message: "Hello, gRPC!"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error calling Echo:", err)
		os.Exit(1)
	}
	fmt.Println("Response from server:", response.Message)

	// Parse the response: "msg|traceparent|traceID|spanID"
	parts := strings.SplitN(response.Message, "|", 4)
	if len(parts) != 4 {
		fmt.Fprintln(os.Stderr, "unexpected response format:", response.Message)
		os.Exit(1)
	}
	msg, tp, trID, spID := parts[0], parts[1], parts[2], parts[3]

	if msg != "Hello, gRPC!" {
		fmt.Fprintln(os.Stderr, "unexpected message:", msg)
		os.Exit(1)
	}
	if !strings.Contains(tp, parentTraceID) {
		fmt.Fprintf(os.Stderr, "trace not propagated: traceparent=%q want trace_id=%s\n", tp, parentTraceID)
		os.Exit(1)
	}
	fmt.Printf("trace propagation OK: traceparent contains trace_id=%s\n", parentTraceID)
	parentSpan.End()

	if trID == "" || spID == "" {
		fmt.Fprintf(os.Stderr, "log correlation missing: traceID=%q spanID=%q\n", trID, spID)
		os.Exit(1)
	}
	fmt.Printf("log correlation OK: trace_id=%s span_id=%s\n", trID, spID)

	bodyStr := scrape("http://127.0.0.1:9373/metrics")
	if !strings.Contains(bodyStr, "rpc_server_request_count") {
		fmt.Fprintln(os.Stderr, "/metrics missing rpc_server_request_count")
		os.Exit(1)
	}
	if !strings.Contains(bodyStr, "rpc_server_request_duration") {
		fmt.Fprintln(os.Stderr, "/metrics missing rpc_server_request_duration")
		os.Exit(1)
	}
	fmt.Println("metrics OK: rpc_server_request_count and rpc_server_request_duration found on :9373")

	mustStatus("http://127.0.0.1:9373/health", http.StatusOK)
	fmt.Println("actuator /health OK")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func scrape(url string) string {
	for range 30 {
		resp, err := http.Get(url)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if strings.Contains(string(b), "go_goroutine_count") {
				return string(b)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scrape failed:", url, err)
		os.Exit(1)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return string(b)
}

func mustStatus(url string, want int) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", url, err)
		os.Exit(1)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != want {
		fmt.Fprintf(os.Stderr, "unexpected status for %s: got %d want %d\n", url, resp.StatusCode, want)
		os.Exit(1)
	}
}

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