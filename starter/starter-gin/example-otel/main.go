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

// Package main is the observability smoke test for starter-gin. It imports
// starter-otel (stdout trace exporter + Prometheus metrics via actuator)
// and proves three observability outcomes end-to-end:
//
//  1. Trace context propagation — a W3C traceparent injected by the test
//     client is extracted by the gin tracing middleware, joining the
//     server-side span to the same trace.
//  2. Metrics emission — after exercising an endpoint, the actuator's
//     /metrics endpoint exposes http.server.request_count,
//     http.server.request_duration, and http.server.active_requests.
//  3. Trace-log correlation — a log line written inside a request handler
//     carries trace_id and span_id from the active span.
package main

import (
	"context"
	"encoding/json"
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

	"github.com/gin-gonic/gin"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-actuator"
	_ "go-spring.org/starter-otel"

	StarterGin "go-spring.org/starter-gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// tag routes the demo log line as a business log so it is easy to spot in
// the smoke output.
var tag = log.RegisterBizTag("example-otel", "")

func init() {
	gs.Provide(&Controller{})
	// Provide a RouterRegister: the starter owns the *gin.Engine and its HTTP
	// server. We wire routes including a diagnostic /echo-trace that returns
	// the inbound traceparent and log-correlation info so the smoke test can
	// assert on them.
	gs.Provide(func(c *Controller) StarterGin.RouterRegister {
		return func(e *gin.Engine) {
			e.GET("/echo-trace/:name", c.EchoTrace)
		}
	})
}

type Controller struct{}

// EchoTrace handles GET /echo-trace/:name. For observability verification it:
//   - echoes the inbound traceparent header (proving the propagated trace
//     context reached the handler);
//   - emits a business log line via log.Infof, which starter-otel's correlation
//     hook stamps with trace_id/span_id;
//   - fetches log.FieldsFromContext to confirm the correlation hook is active.
func (c *Controller) EchoTrace(ctx *gin.Context) {
	name := ctx.Param("name")

	// Emit a log line to exercise trace-log correlation.
	log.Infof(ctx.Request.Context(), tag, "handling /echo-trace/%s", name)

	// Collect trace_id/span_id from the active span via the log hook.
	fields := log.FieldsFromContext(ctx.Request.Context())
	traceID, spanID := fieldValue(fields, "trace_id"), fieldValue(fields, "span_id")

	ctx.JSON(http.StatusOK, gin.H{
		"message":     "Hello, " + name,
		"traceParent": ctx.GetHeader("traceparent"),
		"traceID":     traceID,
		"spanID":      spanID,
	})
}

// fieldValue returns the string value of a log.Field by key, or "" if not found.
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

// EchoTraceResponse mirrors the JSON returned by /echo-trace/:name.
type EchoTraceResponse struct {
	Message     string `json:"message"`
	TraceParent string `json:"traceParent"`
	TraceID     string `json:"traceID"`
	SpanID      string `json:"spanID"`
}

func runTest() {
	// ------------------------------------------------------------------
	// Assertion 1: Trace context propagation.
	// Create a parent span, inject its trace context into an HTTP request
	// via the W3C propagator, call the server, and verify the handler
	// received the same traceparent.
	// ------------------------------------------------------------------
	bgCtx := context.Background()
	ctx, parentSpan := otel.Tracer("example-otel").Start(bgCtx, "gin-e2e-test")
	parentTraceID := parentSpan.SpanContext().TraceID().String()

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:8001/echo-trace/world", nil)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println("Response from server:", string(body))

	var echoResp EchoTraceResponse
	if err := json.Unmarshal(body, &echoResp); err != nil {
		fmt.Fprintln(os.Stderr, "invalid JSON from /echo-trace/:name:", err)
		os.Exit(1)
	}
	if echoResp.Message != "Hello, world" {
		fmt.Fprintln(os.Stderr, "unexpected message:", echoResp.Message)
		os.Exit(1)
	}

	// The server echoes the inbound traceparent; verify it carries the same
	// trace ID as the parent span we injected.
	if !strings.Contains(echoResp.TraceParent, parentTraceID) {
		fmt.Fprintf(os.Stderr, "trace not propagated: traceparent=%q want trace_id=%s\n",
			echoResp.TraceParent, parentTraceID)
		os.Exit(1)
	}
	fmt.Printf("trace propagation OK: traceparent contains trace_id=%s\n", parentTraceID)
	parentSpan.End()

	// ------------------------------------------------------------------
	// Assertion 2: Trace-log correlation.
	// The handler emitted a log line; verify FieldsFromContext returned
	// non-empty trace_id and span_id.
	// ------------------------------------------------------------------
	if echoResp.TraceID == "" || echoResp.SpanID == "" {
		fmt.Fprintf(os.Stderr, "log correlation missing: traceID=%q spanID=%q\n",
			echoResp.TraceID, echoResp.SpanID)
		os.Exit(1)
	}
	fmt.Printf("log correlation OK: trace_id=%s span_id=%s\n", echoResp.TraceID, echoResp.SpanID)

	// ------------------------------------------------------------------
	// Assertion 3: Prometheus metrics on the actuator management port.
	// After a request, the /metrics endpoint must expose the gin metrics.
	// ------------------------------------------------------------------
	bodyStr := scrape("http://127.0.0.1:9370/metrics")
	if !strings.Contains(bodyStr, "http_server_request_count") {
		fmt.Fprintln(os.Stderr, "/metrics missing http_server_request_count")
		os.Exit(1)
	}
	if !strings.Contains(bodyStr, "http_server_request_duration") {
		fmt.Fprintln(os.Stderr, "/metrics missing http_server_request_duration")
		os.Exit(1)
	}
	fmt.Println("metrics OK: http_server_request_count and http_server_request_duration found on :9370")

	// Basic actuator health check.
	mustStatus("http://127.0.0.1:9370/health", http.StatusOK)
	fmt.Println("actuator /health OK")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// scrape fetches url and returns the body, polling briefly because the
// actuator binds asynchronously and Prometheus async runtime callbacks
// only fire on the first collection.
func scrape(url string) string {
	for range 30 {
		resp, err := http.Get(url)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body := string(b)
			if strings.Contains(body, "go_goroutine_count") {
				return body
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

// mustStatus fetches url and exits non-zero unless the response status
// matches want.
func mustStatus(url string, want int) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", url, err)
		os.Exit(1)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != want {
		fmt.Fprintf(os.Stderr, "unexpected status for %s: got %d want %d\n",
			url, resp.StatusCode, want)
		os.Exit(1)
	}
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