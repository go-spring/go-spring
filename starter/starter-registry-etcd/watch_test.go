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

package StarterRegistryEtcd

import (
	"context"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// fakeWatchSource is an in-process discoveryKV whose every watch comes back
// already cancelled by etcd: the stream carries the cancellation and is then
// closed. That is the failure a watch loop has to survive, and making it
// unconditional turns the re-arm into an observable second Watch call.
type fakeWatchSource struct {
	mu     sync.Mutex
	opened int
}

// watches reports how many watch streams have been opened.
func (f *fakeWatchSource) watches() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.opened
}

// Get serves an empty snapshot; the cancelled streams below mean the loop never
// reaches the refresh that would read it.
func (f *fakeWatchSource) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{}, nil
}

// Watch returns a stream that etcd has cancelled — a compaction being the
// everyday cause, and the one clientv3 reports through the response rather than
// by closing the channel.
func (f *fakeWatchSource) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse, 1)
	f.mu.Lock()
	f.opened++
	f.mu.Unlock()
	ch <- clientv3.WatchResponse{CompactRevision: 1}
	close(ch)
	return ch
}

// A watch cancelled by etcd carries no further events and never recovers on its
// own, so the loop has to report the failure and open a new watch. Without that
// the cache would freeze at its last value while every gauge still looked alive
// — a dead discovery looking exactly like a quiet cluster.
func TestWatchLoopReportsCancelAndReArms(t *testing.T) {
	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := &fakeWatchSource{}
	d := &etcdDiscovery{
		client: f, keyPrefix: "/services/",
		bgCtx: bgCtx, entries: map[string]*serviceEntry{},
	}
	done := make(chan struct{})
	go func() { d.watchLoop("orders-watch", d.entry("orders-watch")); close(done) }()

	// The cancellation reaches the freshness metric, not just the log.
	waitFor(t, func() bool {
		v, ok := trySumValue(t, "discovery.sync_total", map[string]string{
			"system": obsSystem, "service": "orders-watch", "status": "failed",
		})
		return ok && v >= 1
	}, "a cancelled watch was never reported as a failed sync")

	// And the loop re-arms instead of returning.
	waitFor(t, func() bool { return f.watches() >= 2 },
		"the cancelled watch was never re-armed")

	// Closing the backend retires the loop rather than leaving it spinning.
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not exit after the backend closed")
	}
}

// A stream that closes without etcd naming a reason is still an ended watch, and
// must not be mistaken for the quiet-cluster case.
func TestWatchLoopReportsClosedStream(t *testing.T) {
	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &etcdDiscovery{
		client: &closedStreamSource{}, keyPrefix: "/services/",
		bgCtx: bgCtx, entries: map[string]*serviceEntry{},
	}
	done := make(chan struct{})
	go func() { d.watchLoop("orders-closed", d.entry("orders-closed")); close(done) }()

	waitFor(t, func() bool {
		v, ok := trySumValue(t, "discovery.sync_total", map[string]string{
			"system": obsSystem, "service": "orders-closed", "status": "failed",
		})
		return ok && v >= 1
	}, "a closed watch stream was never reported as a failed sync")

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not exit after the backend closed")
	}
}

// closedStreamSource is a discoveryKV whose watch closes immediately without a
// cancellation response.
type closedStreamSource struct{}

func (closedStreamSource) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{}, nil
}

func (closedStreamSource) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse)
	close(ch)
	return ch
}
