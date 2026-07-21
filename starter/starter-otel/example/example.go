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

// Command example is the consolidated observability smoke: it imports
// starter-otel (Prometheus metrics exporter) alongside starter-actuator and
// proves the two Task-4 outcomes end to end, with no per-component wiring:
//
//  1. /metrics is served on the actuator's single management port (:9370),
//     not a separate metrics port — starter-otel contributes its Prometheus
//     scrape handler as an endpoint.Endpoint and the actuator mounts it.
//  2. A log line written inside a span carries that span's trace_id/span_id,
//     so a log can be correlated with its trace — starter-otel installs the
//     log.FieldsFromContext hook when tracing is enabled.
package main

import (
	"context"
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
	"go.opentelemetry.io/otel"
)

// tag routes the demo log line as a business log so it is easy to spot in the
// smoke output.
var tag = log.RegisterBizTag("example", "")

func main() {
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling starter examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	go func() {
		time.Sleep(700 * time.Millisecond)
		runTest()
	}()

	gs.Run()
}

// runTest exercises the two consolidated outcomes and exits non-zero on any
// failure, then triggers a graceful shutdown.
func runTest() {
	// (1) Trace ↔ log correlation. Start a span, then confirm the log hook
	// starter-otel installed lifts the span's trace_id/span_id off the context.
	// Emitting a real log line proves the same ids render in the log output.
	ctx, span := otel.Tracer("example").Start(context.Background(), "demo-request")
	log.Infof(ctx, tag, "handling demo request")

	fields := log.FieldsFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	span.End()

	if !hasKey(fields, "trace_id") || !hasKey(fields, "span_id") {
		fmt.Fprintf(os.Stderr, "trace correlation missing: FieldsFromContext=%v\n", fields)
		os.Exit(1)
	}
	if !span.SpanContext().IsValid() {
		fmt.Fprintln(os.Stderr, "span context is not valid; tracing not active")
		os.Exit(1)
	}
	fmt.Printf("log correlation OK: trace_id=%s span_id=%s\n", traceID, spanID)

	// (2) Prometheus /metrics on the actuator management port. metrics.port=0 in
	// the config disables the dedicated scrape server, so this endpoint proves
	// the actuator is serving otel's metrics — a single management port.
	body := scrape("http://127.0.0.1:9370/metrics")
	if !strings.Contains(body, "go_goroutine_count") {
		fmt.Fprintln(os.Stderr, "/metrics did not expose runtime metrics (go_goroutine_count missing)")
		os.Exit(1)
	}
	fmt.Println("actuator /metrics OK: runtime metrics exposed on :9370")

	// The actuator's own endpoints still work alongside the mounted /metrics.
	mustStatus("http://127.0.0.1:9370/health", http.StatusOK)
	fmt.Println("actuator /health OK")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// hasKey reports whether fields contains a field with the given key.
func hasKey(fields []log.Field, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

// scrape fetches url and returns the body, exiting non-zero on error. It polls
// briefly because the actuator binds asynchronously and the Prometheus async
// runtime callbacks only fire on the first collection.
func scrape(url string) string {
	var body string
	for range 30 {
		resp, err := http.Get(url)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(b)
			if strings.Contains(body, "go_goroutine_count") {
				return body
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return body
}

// mustStatus fetches url and exits the process non-zero unless the response
// status matches want.
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

// init sets the working directory to this source file's directory so relative
// config lookups (conf/) resolve against the source location.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
