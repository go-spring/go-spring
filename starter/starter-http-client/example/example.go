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

// Command example is the smoke test for the declarative HTTP client (the
// OpenFeign / @HttpExchange equivalent). It proves the full loop end to end
// with no per-call wiring:
//
//	declare the interface (idl/greet.idl) -> generate the client (proto/) ->
//	inject an assembled *http.Client and call another service.
//
// It exercises the four acceptance outcomes against real in-process backends:
//
//  1. Direct address — the "direct" client is pinned to one backend's addr.
//  2. Service discovery + load balancing — the "discovered" client routes by
//     service name through a static discovery backend and round-robins across
//     two instances (observed via the servedBy field flipping between them).
//  3. Resilience — the "guarded" client points at a backend that always fails;
//     after the error threshold the breaker opens and calls fast-fail with
//     resilience.ErrCircuitOpen instead of hitting the network.
//  4. Trace propagation — a client span injects a W3C traceparent header
//     (starter-http-client's otelhttp base transport + the global propagator);
//     the backend echoes it back, so the same trace id is observable on both
//     ends.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/cloud/resilience"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	_ "go-spring.org/starter-http-client"
	"go-spring.org/starter-http-client/example/proto"
)

// Fixed loopback ports for the three in-process backends. Two healthy greet
// instances prove load balancing; the third always fails, to trip the breaker.
const (
	addrBackendA = "127.0.0.1:9471"
	addrBackendB = "127.0.0.1:9472"
	addrFlaky    = "127.0.0.1:9473"

	// serviceName is the logical name the "discovered" client resolves through
	// the static discovery backend below. It doubles as the resilience resource
	// key for the "guarded" client (the breaker keys on the request host, which
	// is the generated client's Target).
	serviceName = "greet-svc"
)

// Service consumes three declarative clients, each backed by a differently
// assembled *http.Client the starter registered under a group key (see
// conf/app.properties). The generated proto.Client only holds an *http.Client,
// so switching a call between a direct address and a discovered service is a
// pure-config change — the call site never changes.
type Service struct {
	Direct     *http.Client `autowire:"direct"`
	Discovered *http.Client `autowire:"discovered"`
	Guarded    *http.Client `autowire:"guarded"`
}

// greetHandler answers /greet with a small JSON body. servedBy identifies which
// instance replied (so load balancing is observable) and traceParent echoes the
// inbound W3C trace-context header (so trace propagation is observable).
func greetHandler(servedBy string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		traceParent := r.Header.Get("traceparent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w,
			`{"message":"Hello, %s!","servedBy":%q,"traceParent":%q}`,
			name, servedBy, traceParent)
	}
}

// startBackend runs a greet server on addr that tags every reply with servedBy.
func startBackend(addr, servedBy string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/greet", greetHandler(servedBy))
	_ = http.ListenAndServe(addr, mux)
}

// startFlakyBackend runs a greet server that always returns 500, so the
// guarded client's breaker trips after the configured error threshold.
func startFlakyBackend(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/greet", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_ = http.ListenAndServe(addr, mux)
}

func main() {
	// Install a real tracer provider and the W3C propagator so the starter's
	// otelhttp base transport produces a valid client span and injects a
	// traceparent header. Without this the global tracer is a no-op and nothing
	// is propagated.
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample())))
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// A static discovery backend that resolves serviceName to the two healthy
	// instances. Registering it under "static" matches discovery=static in the
	// config; the "discovered" client watches it through the LiveDialer.
	discovery.Register("static", &staticDiscovery{eps: []discovery.Endpoint{
		{Addr: addrBackendA, Healthy: true},
		{Addr: addrBackendB, Healthy: true},
	}})

	go startBackend(addrBackendA, "backend-A")
	go startBackend(addrBackendB, "backend-B")
	go startFlakyBackend(addrFlaky)

	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(svrBean.Interface().(*Service))
	}()

	gs.Run()
}

func runTest(s *Service) {
	ctx := context.Background()

	// (1) Direct address: the "direct" client is pinned to backend-A.
	directClient := &proto.Client{Target: serviceName, HTTPClient: s.Direct}
	_, out, err := directClient.Greet(ctx, &proto.GreetReq{Name: "Ada"})
	if err != nil {
		fail("direct call failed: %v", err)
	}
	if out.Message != "Hello, Ada!" || out.ServedBy != "backend-A" {
		fail("unexpected direct response: %+v", out)
	}
	fmt.Printf("direct OK: message=%q servedBy=%s\n", out.Message, out.ServedBy)

	// (2) Discovery + load balancing: the "discovered" client routes by service
	// name and round-robins, so servedBy flips between the two instances.
	lbClient := &proto.Client{Target: serviceName, HTTPClient: s.Discovered}
	seen := map[string]int{}
	for range 4 {
		_, out, err := lbClient.Greet(ctx, &proto.GreetReq{Name: "Grace"})
		if err != nil {
			fail("discovered call failed: %v", err)
		}
		seen[out.ServedBy]++
	}
	if seen["backend-A"] == 0 || seen["backend-B"] == 0 {
		fail("load balancing did not spread across instances: %v", seen)
	}
	fmt.Printf("discovery+LB OK: round-robin hit %v\n", seen)

	// (3) Trace propagation: start a client span, call through the direct client,
	// and confirm the backend echoed our traceparent carrying the same trace id.
	spanCtx, span := otel.Tracer("example").Start(ctx, "greet-call")
	traceID := span.SpanContext().TraceID().String()
	_, traced, err := directClient.Greet(spanCtx, &proto.GreetReq{Name: "Alan"})
	span.End()
	if err != nil {
		fail("traced call failed: %v", err)
	}
	if traced.TraceParent == "" || !strings.Contains(traced.TraceParent, traceID) {
		fail("trace not propagated: traceparent=%q want trace_id=%s", traced.TraceParent, traceID)
	}
	fmt.Printf("trace propagation OK: trace_id=%s echoed as %q\n", traceID, traced.TraceParent)

	// (4) Resilience: the "guarded" client hits a backend that always 500s. The
	// first calls fail against the network; once the breaker opens, later calls
	// fast-fail with ErrCircuitOpen without a round-trip.
	guardedClient := &proto.Client{Target: serviceName, HTTPClient: s.Guarded}
	var breakerOpened bool
	for i := range 6 {
		start := time.Now()
		_, _, err := guardedClient.Greet(ctx, &proto.GreetReq{Name: "Edsger"})
		if err == nil {
			fail("guarded call unexpectedly succeeded against a failing backend")
		}
		if errors.Is(err, resilience.ErrCircuitOpen) {
			breakerOpened = true
			fmt.Printf("resilience OK: breaker open on attempt %d, fast-failed in %s\n", i+1, time.Since(start))
			break
		}
	}
	if !breakerOpened {
		fail("breaker never opened despite repeated failures")
	}

	fmt.Println("all declarative HTTP client checks passed")
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// fail logs a fatal message and exits non-zero, marking the smoke test failed.
func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}

// staticDiscovery is a fixed, in-memory discovery.Discovery for the smoke test:
// it always resolves to the same endpoint set and never pushes updates.
type staticDiscovery struct {
	eps []discovery.Endpoint
}

func (d *staticDiscovery) Resolve(context.Context, string) ([]discovery.Endpoint, error) {
	return d.eps, nil
}

func (d *staticDiscovery) Watch(context.Context, string) (discovery.Watcher, error) {
	return &staticWatcher{eps: d.eps, done: make(chan struct{})}, nil
}

// staticWatcher yields the initial snapshot once, then blocks until stopped —
// enough for the LiveDialer to seed its endpoint set for load balancing.
type staticWatcher struct {
	eps      []discovery.Endpoint
	done     chan struct{}
	sent     bool
	stopOnce sync.Once
}

func (w *staticWatcher) Next() ([]discovery.Endpoint, error) {
	if !w.sent {
		w.sent = true
		return w.eps, nil
	}
	<-w.done
	return nil, context.Canceled
}

func (w *staticWatcher) Stop() error {
	w.stopOnce.Do(func() { close(w.done) })
	return nil
}

// init sets the working directory to this source file's directory so relative
// config lookups (conf/) resolve against the source location.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		if err := os.Chdir(filepath.Dir(filename)); err != nil {
			panic(err)
		}
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
