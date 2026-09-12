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

package StarterGovernanceEtcd

import (
	"context"
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/governance"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const govRulesV1 = `govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
`

const govRulesV2 = `govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 300ms
    max-retries: 1
`

// fakeKV stands in for a live etcd: it serves one value and pipes synthetic
// watch events to the stream the source opened.
type fakeKV struct {
	val   string
	watch chan clientv3.WatchResponse
}

func (f *fakeKV) Get(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{
		Kvs: []*mvccpb.KeyValue{{Value: []byte(f.val)}},
	}, nil
}

func (f *fakeKV) Watch(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan {
	return f.watch
}

func (f *fakeKV) Close() error { return nil }

// put simulates an etcd PUT on the watched key.
func (f *fakeKV) put(val string) {
	f.watch <- clientv3.WatchResponse{
		Events: []*clientv3.Event{{
			Type: clientv3.EventTypePut,
			Kv:   &mvccpb.KeyValue{Value: []byte(val)},
		}},
	}
}

func newGovTestSource(t *testing.T, val string) (*EtcdSource, *fakeKV) {
	t.Helper()
	fake := &fakeKV{val: val, watch: make(chan clientv3.WatchResponse, 8)}
	src, err := NewEtcdSource(fake, "/govern.yaml", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	return src, fake
}

// TestEtcdSource_PushChain covers the direct-watch chain: seed, PUT → parse →
// push, bad value keeps last good, byte-equal re-put is a no-op.
func TestEtcdSource_PushChain(t *testing.T) {
	src, fake := newGovTestSource(t, govRulesV1)
	if err := src.Init(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()

	var mu sync.Mutex
	var pushes int
	var pushed governance.Config
	src.Subscribe(func(cfg governance.Config) { mu.Lock(); pushed = cfg; pushes++; mu.Unlock() })

	if cfg := src.Snapshot(); cfg.Default.AttemptTimeout != 100*time.Millisecond {
		t.Fatalf("seed snapshot: %+v", cfg.Default)
	}

	// A PUT with new rules pushes.
	fake.put(govRulesV2)
	time.Sleep(100 * time.Millisecond) // the watch goroutine delivers async
	mu.Lock()
	got, n := pushed.Default.AttemptTimeout, pushes
	mu.Unlock()
	if n != 1 || got != 300*time.Millisecond {
		t.Fatalf("put should push new rules once: n=%d timeout=%v", n, got)
	}

	// A bad value keeps the last good snapshot and pushes nothing.
	fake.put("govern: { broken")
	time.Sleep(100 * time.Millisecond)
	if cfg := src.Snapshot(); cfg.Default.AttemptTimeout != 300*time.Millisecond {
		t.Fatalf("bad put must keep last good snapshot: %+v", cfg.Default)
	}
	mu.Lock()
	n = pushes
	mu.Unlock()
	if n != 1 {
		t.Fatalf("bad put must not push: n=%d", n)
	}

	// A byte-equal re-put is a no-op.
	fake.put(govRulesV2)
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	n = pushes
	mu.Unlock()
	if n != 1 {
		t.Fatalf("byte-equal re-put must not push: n=%d", n)
	}
}

// TestEtcdSource_BadSeedFailsFast pins startup behavior: an unparseable
// initial value fails construction instead of arming a disabled center.
func TestEtcdSource_BadSeedFailsFast(t *testing.T) {
	fake := &fakeKV{val: "govern: {", watch: make(chan clientv3.WatchResponse, 1)}
	if _, err := NewEtcdSource(fake, "/govern.yaml", "yaml"); err == nil {
		t.Fatal("bad initial value should fail construction")
	}
}
