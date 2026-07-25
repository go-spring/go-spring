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

// Package main is the observability smoke test for starter-hertz. It proves:
//   1. Trace context propagation (W3C traceparent round-trip).
//   2. Metrics emission (http.server.request_count etc. on actuator /metrics).
//   3. Trace-log correlation (trace_id/span_id in log fields).
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

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-actuator"
	_ "go-spring.org/starter-otel"

	StarterHertz "go-spring.org/starter-hertz"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

var tag = log.RegisterBizTag("example-otel", "")

func init() {
	gs.Provide(&Controller{})
	gs.Provide(func(c *Controller) StarterHertz.RouterRegister {
		return func(h *server.Hertz) {
			h.GET("/echo-trace/:name", c.EchoTrace)
		}
	})
}

type Controller struct{}

func (c *Controller) EchoTrace(_ context.Context, ctx *app.RequestContext) {
	name := ctx.Param("name")
	// Hertz's *app.RequestContext does not implement context.Context;
	// use the underlying Go context for log/trace operations.
	reqCtx := ctx.GetContext()
	log.Infof(reqCtx, tag, "handling /echo-trace/%s", name)

	fields := log.FieldsFromContext(reqCtx)
	traceID, spanID := fieldValue(fields, "trace_id"), fieldValue(fields, "span_id")

	ctx.JSON(http.StatusOK, map[string]string{
		"message":     "Hello, " + name,
		"traceParent": string(ctx.GetHeader("traceparent")),
		"traceID":     traceID,
		"spanID":      spanID,
	})
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
	ctx, parentSpan := otel.Tracer("example-otel").Start(bgCtx, "hertz-e2e-test")
	parentTraceID := parentSpan.SpanContext().TraceID().String()

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:8003/echo-trace/world", nil)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "request failed:", err)
		os.Exit(1)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println("Response from server:", string(body))

	var m map[string]string
	if err := json.Unmarshal(body, &m); err != nil {
		fmt.Fprintln(os.Stderr, "invalid JSON:", err)
		os.Exit(1)
	}
	if m["message"] != "Hello, world" {
		fmt.Fprintln(os.Stderr, "unexpected message:", m["message"])
		os.Exit(1)
	}
	if !strings.Contains(m["traceParent"], parentTraceID) {
		fmt.Fprintf(os.Stderr, "trace not propagated: traceparent=%q want trace_id=%s\n", m["traceParent"], parentTraceID)
		os.Exit(1)
	}
	fmt.Printf("trace propagation OK: traceparent contains trace_id=%s\n", parentTraceID)
	parentSpan.End()

	if m["traceID"] == "" || m["spanID"] == "" {
		fmt.Fprintf(os.Stderr, "log correlation missing: traceID=%q spanID=%q\n", m["traceID"], m["spanID"])
		os.Exit(1)
	}
	fmt.Printf("log correlation OK: trace_id=%s span_id=%s\n", m["traceID"], m["spanID"])

	bodyStr := scrape("http://127.0.0.1:9372/metrics")
	if !strings.Contains(bodyStr, "http_server_request_count") {
		fmt.Fprintln(os.Stderr, "/metrics missing http_server_request_count")
		os.Exit(1)
	}
	if !strings.Contains(bodyStr, "http_server_request_duration") {
		fmt.Fprintln(os.Stderr, "/metrics missing http_server_request_duration")
		os.Exit(1)
	}
	fmt.Println("metrics OK: http_server_request_count and http_server_request_duration found on :9372")

	mustStatus("http://127.0.0.1:9372/health", http.StatusOK)
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