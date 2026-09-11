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
	"encoding/json"
	"sync"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// instance is the process advertisement the registrar publishes; it is the
// neutral [discovery.Instance] contract. The crash-safety contract every
// registry starter follows lives in starter/DESIGN §3 (Register must
// self-renew so correctness never depends on Deregister).
type instance = discovery.Instance

// instanceValue is the JSON payload stored at an instance key. A discovery
// backend reading the same prefix reconstructs an Endpoint from it.
type instanceValue struct {
	ServiceName string            `json:"service_name"`
	Addr        string            `json:"addr"`
	Weight      int               `json:"weight,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// etcdRegistrar publishes instances to an etcd cluster. Each instance is written
// under a key bound to its own lease and kept alive by a background keep-alive;
// if the process dies the lease expires and etcd deletes the key automatically.
// If the keep-alive dies instead (etcd restart, lease expired server-side), the
// registrar re-grants a lease and re-puts the key with exponential backoff, so
// the entry always comes back without operator action.
type etcdRegistrar struct {
	client    *clientv3.Client
	keyPrefix string
	ttlSecs   int64

	// backoffBase/backoffCap pace the re-register retry loop after a keep-alive
	// loss: 1s doubling up to 1min. Fields (not constants) so tests shrink them.
	backoffBase time.Duration
	backoffCap  time.Duration

	// publish is the grant+put+keepalive step, a field so tests can fake the
	// cluster side of the self-healing loop without an etcd server.
	publish func(h *hold) (<-chan *clientv3.LeaseKeepAliveResponse, error)

	mu    sync.Mutex
	holds map[string]*hold // instance key -> its lease keep-alive
}

// hold tracks the lease and keep-alive goroutine backing one registered key,
// plus the last payload written under it so a weight update can rewrite the
// full instanceValue without rebuilding the lease — and so the self-healing
// loop can re-register the current value after a keep-alive loss.
type hold struct {
	leaseID clientv3.LeaseID
	cancel  context.CancelFunc
	reg     instance

	// done is closed by stop to end the keep-alive drain and the re-register
	// retry loop; stopOnce makes stop idempotent (Deregister + re-Register).
	done     chan struct{}
	stopOnce sync.Once
}

// newHold builds a hold for reg with its stop signal wired up.
func newHold(reg instance) *hold {
	return &hold{reg: reg, done: make(chan struct{})}
}

// stop cancels the keep-alive context and signals the watcher goroutine to
// exit. It is idempotent and safe to call from any goroutine; cancel may be
// nil if a concurrent publish has not stored it yet — publish also checks
// done after storing, so a stop racing publish still cancels the keep-alive.
func (h *hold) stop() {
	h.stopOnce.Do(func() {
		if h.cancel != nil {
			h.cancel()
		}
		close(h.done)
	})
}

// stopped reports whether stop has been called (Deregister or retirement).
func (h *hold) stopped() bool {
	select {
	case <-h.done:
		return true
	default:
		return false
	}
}

// stopHold stops h under the registrar lock: hold fields (leaseID, cancel) are
// written by publish under the same lock, so a stop racing a re-publish never
// misses the cancel func.
func (r *etcdRegistrar) stopHold(h *hold) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h.stop()
}

// newEtcdRegistrar returns a registrar writing through cli (the shared center
// client; the cluster was already probed when cli was built) with c's prefix
// and TTL. It does NOT close cli — the owner (etcdCenter) does.
func newEtcdRegistrar(c EtcdConfig, cli *clientv3.Client) (*etcdRegistrar, error) {
	if cli == nil {
		return nil, errutil.Explain(nil, "registry-etcd: nil etcd client")
	}
	r := &etcdRegistrar{
		client:      cli,
		keyPrefix:   c.KeyPrefix,
		ttlSecs:     c.ttlSeconds(),
		backoffBase: time.Second,
		backoffCap:  time.Minute,
		holds:       map[string]*hold{},
	}
	r.publish = r.etcdPublish
	return r, nil
}

// instanceID returns the instance id within the service: the caller-supplied ID,
// or a stable one derived from the service name and advertised address.
func instanceID(reg instance) string {
	if reg.ID != "" {
		return reg.ID
	}
	return reg.ServiceName + "-" + reg.Addr
}

// keyFor returns the etcd key an instance is written under: prefix + service +
// "/" + instance id.
func (r *etcdRegistrar) keyFor(reg instance) string {
	return r.keyPrefix + reg.ServiceName + "/" + instanceID(reg)
}

// Register grants a lease, writes the instance under it, and starts a keep-alive
// so the entry stays live until Deregister or process death. A watcher goroutine
// drains keep-alive renewals and, if the keep-alive channel closes (etcd
// restart, lease lost), re-registers with backoff so the key never silently
// vanishes. Registering the same instance again refreshes it: the previous
// lease is revoked first.
func (r *etcdRegistrar) Register(ctx context.Context, reg instance) error {
	if err := errutil.RequireField("registry-etcd", "addr", reg.Addr); err != nil {
		return err
	}
	// An unset (or misconfigured negative) weight is normalized at write time
	// so "default" is never stored as 0 — 0 is reserved for the runtime drain
	// signal, only reachable through UpdateWeight.
	if reg.Weight <= 0 {
		reg.Weight = 1
	}

	h := newHold(reg)
	ka, err := r.publish(h)
	if err != nil {
		return err
	}

	key := r.keyFor(reg)
	r.mu.Lock()
	// Re-registering the same instance refreshes it: retire the old lease.
	// (Already under r.mu, so retire inline rather than via stopHold.)
	if old, ok := r.holds[key]; ok {
		old.stop()
		_, _ = r.client.Revoke(context.Background(), old.leaseID)
	}
	r.holds[key] = h
	r.mu.Unlock()

	go r.watchKeepAlive(key, h, ka)
	return nil
}

// etcdPublish grants a fresh lease, puts reg's key under it, and starts the
// keep-alive whose renewal channel the caller must drain. It stores the new
// lease and cancel func in h so UpdateWeight writes ride the current lease and
// stop cancels the live keep-alive. It is the single (re-)registration step
// used by both Register and the self-healing loop.
func (r *etcdRegistrar) etcdPublish(h *hold) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	// Snapshot the payload under the lock: UpdateWeight may rewrite h.reg
	// concurrently, and a re-publish must carry the latest advertised weight.
	r.mu.Lock()
	reg := h.reg
	r.mu.Unlock()
	val, err := json.Marshal(instanceValue{
		ServiceName: reg.ServiceName,
		Addr:        reg.Addr,
		Weight:      reg.Weight,
		Metadata:    reg.Metadata,
	})
	if err != nil {
		return nil, errutil.Explain(err, "registry-etcd: marshal instance %q", reg.ServiceName)
	}
	// The publish step runs detached from Register's ctx (the self-healing loop
	// calls it from its own goroutine), so bound each etcd call by the lease
	// TTL — a dead cluster fails this step rather than blocking forever.
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.ttlSecs)*time.Second)
	defer cancel()
	grant, err := r.client.Grant(ctx, r.ttlSecs)
	if err != nil {
		return nil, errutil.Explain(err, "registry-etcd: grant lease for %q", reg.ServiceName)
	}
	key := r.keyFor(reg)
	if _, err := r.client.Put(ctx, key, string(val), clientv3.WithLease(grant.ID)); err != nil {
		_, _ = r.client.Revoke(context.Background(), grant.ID)
		return nil, errutil.Explain(err, "registry-etcd: put %q", key)
	}

	// KeepAlive runs until its context is cancelled (on stop). The returned
	// channel must be drained or the lease will not be renewed.
	kaCtx, kaCancel := context.WithCancel(context.Background())
	ka, err := r.client.KeepAlive(kaCtx, grant.ID)
	if err != nil {
		kaCancel()
		_, _ = r.client.Revoke(context.Background(), grant.ID)
		return nil, errutil.Explain(err, "registry-etcd: keepalive for %q", reg.ServiceName)
	}

	r.mu.Lock()
	h.leaseID = grant.ID
	h.cancel = kaCancel
	r.mu.Unlock()
	// A stop that raced this publish (before cancel was stored) could not
	// cancel the keep-alive context; catch up so it never leaks.
	if h.stopped() {
		kaCancel()
	}
	return ka, nil
}

// watchKeepAlive drains keep-alive renewals for one hold. The channel closes
// when the keep-alive dies (etcd restart, lease expired server-side, or the
// local stop). Unless the hold was stopped on purpose (Deregister /
// re-Register), that means the registered key will vanish once the TTL
// elapses, so it re-runs the publish step with exponential backoff (base 1s
// doubling, capped at 1min) until the instance is registered again.
func (r *etcdRegistrar) watchKeepAlive(key string, h *hold, ka <-chan *clientv3.LeaseKeepAliveResponse) {
	for {
		// Drain renewals; the lease is kept alive as long as we consume them.
		// The channel closing is the keep-alive death signal.
		for range ka {
		}
		if h.stopped() {
			return
		}
		log.Errorf(context.Background(), starterTag,
			"keepalive for key=%s died (etcd unreachable or lease lost); re-registering with backoff", key)
		backoff := r.backoffBase
		for {
			if h.stopped() {
				return
			}
			nka, err := r.publish(h)
			if err == nil {
				log.Infof(context.Background(), starterTag, "re-registered key=%s under a new lease", key)
				ka = nka
				break
			}
			log.Errorf(context.Background(), starterTag,
				"re-register key=%s failed: %v; retrying in %s", key, err, backoff)
			select {
			case <-h.done:
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > r.backoffCap {
				backoff = r.backoffCap
			}
		}
	}
}

// UpdateWeight rewrites reg's key with a new weight on its existing lease —
// no lease is granted, revoked or re-kept-alive, so the instance never
// disappears from discovery mid-update and watchers simply see a new value
// for the same key. It fails if reg was never registered through Register.
func (r *etcdRegistrar) UpdateWeight(ctx context.Context, reg instance, weight int) error {
	key := r.keyFor(reg)
	r.mu.Lock()
	h, ok := r.holds[key]
	r.mu.Unlock()
	if !ok {
		return errutil.Explain(nil, "registry-etcd: update weight for unregistered instance %q", key)
	}
	updated := h.reg
	updated.Weight = weight
	val, err := json.Marshal(instanceValue{
		ServiceName: updated.ServiceName,
		Addr:        updated.Addr,
		Weight:      updated.Weight,
		Metadata:    updated.Metadata,
	})
	if err != nil {
		return errutil.Explain(err, "registry-etcd: marshal instance %q", updated.ServiceName)
	}
	if _, err := r.client.Put(ctx, key, string(val), clientv3.WithLease(h.leaseID)); err != nil {
		return errutil.Explain(err, "registry-etcd: update weight put %q", key)
	}
	r.mu.Lock()
	h.reg = updated
	r.mu.Unlock()
	return nil
}

// Deregister stops the keep-alive and revokes the lease, which deletes the key.
// It is idempotent: deregistering an instance that is not registered is a no-op.
func (r *etcdRegistrar) Deregister(ctx context.Context, reg instance) error {
	key := r.keyFor(reg)
	r.mu.Lock()
	h, ok := r.holds[key]
	if ok {
		delete(r.holds, key)
	}
	r.mu.Unlock()
	if !ok {
		return nil
	}
	r.stopHold(h)
	if _, err := r.client.Revoke(ctx, h.leaseID); err != nil {
		return errutil.Explain(err, "registry-etcd: revoke lease for %q", reg.ServiceName)
	}
	return nil
}
