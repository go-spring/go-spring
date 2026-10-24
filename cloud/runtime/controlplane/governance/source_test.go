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

package governance

import (
	"context"
	"testing"

	"go-spring.org/cloud/runtime/dataplane/inbound/admission"
	"go-spring.org/cloud/runtime/dataplane/outbound/resilience"
	"go-spring.org/log"
)

// TestSource_PushSourceDrivesCenter covers the custom-source end-to-end path:
// a PushSource handed to NewCenter arms the center from its snapshot and later
// pushes fan out to the module authorities — the same fan-out any source drives.
func TestSource_PushSourceDrivesCenter(t *testing.T) {
	src := pushSource(timeoutConfig(100))
	resClient, lb, fc, fs := newAuthorities()
	if _, err := NewCenter(resClient, admission.NewManager(), lb, fc, fs, src); err != nil {
		t.Fatal(err)
	}
	if p := resClient.PolicyFor("redis:cache"); p.Timeout != dur(100) {
		t.Fatalf("snapshot policy: want 100ms, got %v", p.Timeout)
	}

	var got resilience.Policy
	resClient.Subscribe("redis:cache", func(p resilience.Policy) { got = p })
	src.Push(context.Background(), timeoutConfig(200))
	if got.Timeout != dur(200) {
		t.Fatalf("push should fan out: want 200ms, got %v", got.Timeout)
	}
	if p := resClient.PolicyFor("redis:cache"); p.Timeout != dur(200) {
		t.Fatalf("post-push policy: want 200ms, got %v", p.Timeout)
	}
}

// TestPushSource_Concurrent runs Push/Snapshot/Subscribe concurrently; run
// with -race, this guards the lock discipline of the ready-made source.
func TestPushSource_Concurrent(t *testing.T) {
	p := pushSource(Config{})
	done := make(chan struct{})
	go func() { defer close(done); p.Subscribe(func(context.Context, Config) {}) }()
	for i := range 100 {
		p.Push(context.Background(), timeoutConfig(i))
		_ = p.Snapshot()
	}
	<-done
	p.Push(context.Background(), timeoutConfig(1))
}

// TestDestroy_ClosesCloseableSource pins Destroy's optional-close contract:
// a source implementing Close is closed; one without Close is left alone.
func TestDestroy_ClosesCloseableSource(t *testing.T) {
	// With a closeable source.
	resClient, lb, fc, fs := newAuthorities()
	src := newCloseableSource(Config{})
	c, err := NewCenter(resClient, admission.NewManager(), lb, fc, fs, src)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if !src.closed {
		t.Fatal("Destroy should close a closeable source")
	}

	// With a plain PushSource (no Close method): Destroy is a no-op.
	resClient2, lb2, fc2, fs2 := newAuthorities()
	c2, err := NewCenter(resClient2, admission.NewManager(), lb2, fc2, fs2, pushSource(Config{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.Close(); err != nil {
		t.Fatal(err)
	}
}

// closeableSource is a Source that also implements Close, to exercise
// Destroy's type assertion.
type closeableSource struct {
	Push   *PushSource
	closed bool
}

func newCloseableSource(cfg Config) *closeableSource {
	return &closeableSource{Push: pushSource(cfg)}
}

func (s *closeableSource) Snapshot() Config                                   { return s.Push.Snapshot() }
func (s *closeableSource) Subscribe(cb func(ctx context.Context, cfg Config)) { s.Push.Subscribe(cb) }
func (s *closeableSource) Close() error                                       { s.closed = true; return nil }

// TestPushDeliversTheChangesContext proves the change's context survives the
// seam unchanged: the subscriber - and through it the center - logs what it did
// about the change against the identity the source named it with.
func TestPushDeliversTheChangesContext(t *testing.T) {
	p := pushSource(Config{})
	var got context.Context
	p.Subscribe(func(ctx context.Context, _ Config) { got = ctx })

	change := log.RootFields(log.String("trace_id", "t1"))
	p.Push(change, timeoutConfig(100))

	if got != change {
		t.Fatal("the pushed context must reach the subscriber unchanged")
	}
	if fields := log.CarriedFields(got); len(fields) != 1 || fields[0].Key != "trace_id" {
		t.Fatalf("the change's fields must ride along: %+v", fields)
	}
}
