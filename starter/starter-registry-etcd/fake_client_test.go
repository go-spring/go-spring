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
	"errors"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// fakeKV is an in-process registrarKV: it stands in for the etcd lease surface
// the registrar writes through, and records what reached it, so a test can
// assert on the write itself and not only on the retry loop's side effects.
//
// The discovery half needs no double — its failure path is driven against a
// real client pointed at a closed port (observe_test.go).
//
// Scripting the failures (grantErrs, grantDown) is a plain field write and must
// happen before the goroutine under test starts. The accessors lock, because
// the registrar's own goroutines read those fields concurrently.
type fakeKV struct {
	mu sync.Mutex

	// grantErrs makes the next Grant calls fail in order, one error each, before
	// Grant succeeds again — an etcd that goes down and comes back.
	grantErrs []error
	// grantDown fails every Grant while set — an etcd that stays down for the
	// whole test.
	grantDown bool

	lease clientv3.LeaseID
	last  map[string]string
	nPuts int
}

// newFakeKV returns a fake etcd with nothing scripted: every call succeeds.
func newFakeKV() *fakeKV {
	return &fakeKV{last: map[string]string{}}
}

// grantRemaining reports how many scripted Grant failures are still unconsumed.
func (f *fakeKV) grantRemaining() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.grantErrs)
}

// writes reports how many values reached the center: Register, UpdateWeight and
// every successful self-heal each write once.
func (f *fakeKV) writes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.nPuts
}

// lastPut returns the value most recently written under key.
func (f *fakeKV) lastPut(key string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.last[key]
	return v, ok
}

// Grant hands out a fresh lease id, or the scripted failure.
func (f *fakeKV) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.grantDown {
		return nil, errors.New("etcd down")
	}
	if len(f.grantErrs) > 0 {
		err := f.grantErrs[0]
		f.grantErrs = f.grantErrs[1:]
		return nil, err
	}
	f.lease++
	return &clientv3.LeaseGrantResponse{ID: f.lease, TTL: ttl}, nil
}

// Put records the value under key. The lease is recorded only through the
// calls the registrar makes on it (Revoke), which is all the tests assert on.
func (f *fakeKV) Put(ctx context.Context, key, val string, lease clientv3.LeaseID) (*clientv3.PutResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.last[key] = val
	f.nPuts++
	return &clientv3.PutResponse{}, nil
}

// Revoke succeeds unconditionally: a deregistered key simply stops being
// written, which is what the tests observe.
func (f *fakeKV) Revoke(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	return &clientv3.LeaseRevokeResponse{}, nil
}

// KeepAlive mirrors the two parts of clientv3's contract the registrar depends
// on: the returned channel must be drained to hold the lease, and cancelling
// ctx closes it. watchKeepAlive reads that close as "the lease is gone", so the
// fake has to end the channel the way the real client ends it — otherwise every
// test would have to close it by hand and would stop proving anything about the
// stop path. No renewals are ever sent, since the registrar only drains.
func (f *fakeKV) KeepAlive(ctx context.Context, id clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	ch := make(chan *clientv3.LeaseKeepAliveResponse)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

// newTestRegistrar wires a registrar over client with test-sized backoff. Tests
// build through here rather than through a struct literal so the construction
// defaults live in one place: a literal skips newEtcdRegistrar entirely, which
// is how the live weight test ended up with a zero backoff — a retry loop that
// spins hot instead of pacing.
func newTestRegistrar(client registrarKV, keyPrefix string) *etcdRegistrar {
	r, err := newEtcdRegistrar(EtcdConfig{KeyPrefix: keyPrefix}, client)
	if err != nil {
		panic(err)
	}
	r.backoffBase = 5 * time.Millisecond
	r.backoffCap = 20 * time.Millisecond
	return r
}
