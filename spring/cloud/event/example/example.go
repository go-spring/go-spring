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

// Command example demonstrates wiring the in-process event bus (stdlib/event)
// through the Go-Spring container.
//
// It models a "configuration changed" domain event delivered to three listener
// beans with zero coupling between them:
//
//   - validateListener (synchronous, order 0) runs first;
//   - applyListener    (synchronous, order 10) runs after it — WithOrder gives
//     deterministic sequencing;
//   - auditListener    (asynchronous) records off the publishing goroutine so a
//     slow audit sink never stalls the publisher.
//
// The integration is declarative: each listener is a bean exported as
// event.Listener, the container collects the whole set, and the demo Runner
// registers them onto the shared event.Bus bean before publishing — the same
// "collect by Export" convention starter-actuator uses for health.Indicator.
// The bus itself is a bean with a Destroy hook so its async workers drain and
// stop on shutdown. No external services are needed; the run self-asserts and
// exits non-zero on failure.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"syscall"
	"time"

	"go-spring.org/spring/gs"
	"go-spring.org/spring/cloud/event"
)

// ConfigChanged is a domain event published when a configuration value changes.
// It is a plain concrete struct — the Go-idiomatic event shape the bus routes by.
type ConfigChanged struct {
	Key   string
	Value string
}

// recorder is a shared, concurrency-safe sink the listeners write to and the
// demo Runner reads back to verify delivery. It is a bean injected into every
// listener and into the demo.
type recorder struct {
	mu    sync.Mutex
	steps []string
	audit chan string
}

func newRecorder() *recorder {
	return &recorder{audit: make(chan string, 4)}
}

func (r *recorder) add(step string) {
	r.mu.Lock()
	r.steps = append(r.steps, step)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.steps)
}

// validateListener subscribes synchronously with order 0, so it runs before
// applyListener for the same event.
type validateListener struct {
	Rec *recorder `autowire:""`
}

func (l *validateListener) Register(bus event.Bus) {
	event.Subscribe(bus, func(_ context.Context, e ConfigChanged) error {
		l.Rec.add("validate:" + e.Key)
		return nil
	}, event.WithOrder(0))
}

// applyListener subscribes synchronously with order 10, so it runs after
// validateListener.
type applyListener struct {
	Rec *recorder `autowire:""`
}

func (l *applyListener) Register(bus event.Bus) {
	event.Subscribe(bus, func(_ context.Context, e ConfigChanged) error {
		l.Rec.add("apply:" + e.Key)
		return nil
	}, event.WithOrder(10))
}

// auditListener subscribes asynchronously: its handler runs on a dedicated
// worker goroutine, so a slow audit sink cannot stall the publisher. Its error
// (if any) is routed via WithErrorHandler since it cannot reach Publish.
type auditListener struct {
	Rec *recorder `autowire:""`
}

func (l *auditListener) Register(bus event.Bus) {
	event.SubscribeAsync(bus, func(_ context.Context, e ConfigChanged) error {
		l.Rec.audit <- "audit:" + e.Key
		return nil
	}, event.WithErrorHandler(func(_ context.Context, err error) {
		fmt.Fprintln(os.Stderr, "audit failed:", err)
	}))
}

// demo is the application Runner. It collects every event.Listener bean, wires
// them onto the shared bus, then publishes a ConfigChanged and asserts the
// synchronous handlers ran in order and the asynchronous audit fired.
type demo struct {
	Bus       event.Bus        `autowire:""`
	Rec       *recorder        `autowire:""`
	Listeners []event.Listener `autowire:"?"`
}

// Run registers the listener beans and kicks off the self-test. It returns
// immediately (a Runner must not block); the verification and shutdown happen on
// a background goroutine.
func (a *demo) Run(ctx context.Context) error {
	for _, l := range a.Listeners {
		l.Register(a.Bus)
	}
	fmt.Printf("registered %d listener(s)\n", len(a.Listeners))

	go a.selfTest()
	return nil
}

func (a *demo) selfTest() {
	// Give gs.Run time to finish starting and install its SIGTERM handler before
	// we trigger shutdown below, so the signal is caught for a graceful drain
	// rather than terminating the process by default disposition.
	time.Sleep(300 * time.Millisecond)

	if err := a.Bus.Publish(context.Background(), ConfigChanged{Key: "log.level", Value: "debug"}); err != nil {
		fail("publish returned error: %v", err)
	}

	// Synchronous handlers have already run inline, in WithOrder sequence.
	steps := a.Rec.snapshot()
	want := []string{"validate:log.level", "apply:log.level"}
	if !slices.Equal(steps, want) {
		fail("sync order mismatch: got %v want %v", steps, want)
	}
	fmt.Println("sync listeners ran in order:", steps)

	// The asynchronous audit fires on its own goroutine; wait briefly for it.
	select {
	case got := <-a.Rec.audit:
		if got != "audit:log.level" {
			fail("unexpected audit payload: %q", got)
		}
		fmt.Println("async listener delivered:", got)
	case <-time.After(2 * time.Second):
		fail("timed out waiting for async audit")
	}

	fmt.Println("OK")
	// Trigger graceful shutdown; the bus bean's Destroy hook drains and stops the
	// async worker.
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func init() {
	// The shared recorder, injected into the listeners and the demo Runner.
	gs.Provide(newRecorder)

	// The event bus bean. Its Destroy hook closes the bus so async workers drain
	// and stop cleanly on shutdown.
	gs.Provide(event.New).Destroy(func(b event.Bus) error { return b.Close() })

	// Listener beans, each exported as event.Listener so the container collects
	// them without any per-listener registration API. Distinct names keep the
	// unnamed-bean default from colliding when several share the same export.
	gs.Provide(&validateListener{}).Name("validateListener").Export(gs.As[event.Listener]())
	gs.Provide(&applyListener{}).Name("applyListener").Export(gs.As[event.Listener]())
	gs.Provide(&auditListener{}).Name("auditListener").Export(gs.As[event.Listener]())

	// The application Runner.
	gs.Provide(&demo{}).Export(gs.As[gs.Runner]())
}

func main() {
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	gs.Run()
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory to the directory of this source file so
// relative config lookups (conf/) resolve against the source location rather
// than the process launch path.
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
