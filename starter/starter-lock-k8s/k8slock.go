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

package StarterLockK8s

import (
	"context"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"go-spring.org/spring/cloud/lock"
	"go-spring.org/stdlib/errutil"
)

// k8sLocker implements [lock.Locker] on top of coordination.k8s.io/Lease
// objects. It owns one Kubernetes clientset shared across acquisitions; every
// acquired lock maps to a single Lease (name = KeyPrefix+key) and runs its own
// renewal goroutine so its lease and Lost() channel are independent of other
// holds.
//
// Compared with the etcd/consul/redis backends this needs no external
// middleware: the Lease API is part of every Kubernetes control plane, which is
// the standard way to elect a leader in-cluster (mirroring what kube-controller
// -manager and spring-cloud-kubernetes do).
type k8sLocker struct {
	client    kubernetes.Interface
	namespace string
	keyPrefix string
}

// newK8sLocker builds a Locker from c, creating the shared clientset eagerly so
// a missing ServiceAccount or bad kubeconfig fails at boot rather than on the
// first Acquire.
func newK8sLocker(c Config) (*k8sLocker, error) {
	client, err := buildClient(c)
	if err != nil {
		return nil, err
	}
	return newK8sLockerWithClient(client, c.Namespace, c.KeyPrefix), nil
}

// newK8sLockerWithClient builds a Locker around an already-constructed client.
// It is the seam tests use to inject a client-go fake clientset instead of a
// live cluster.
func newK8sLockerWithClient(client kubernetes.Interface, namespace, keyPrefix string) *k8sLocker {
	return &k8sLocker{
		client:    client,
		namespace: namespace,
		keyPrefix: keyPrefix,
	}
}

// Acquire blocks until the Lease for key is held, ctx is done, or a
// non-retriable backend error occurs. Contention is retried every
// RetryInterval; a transient API error aborts, matching the abstraction's
// contract that only genuine contention should be retried silently.
func (l *k8sLocker) Acquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, error) {
	o := lock.Apply(opts...)
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		held, ok, err := l.tryOnce(ctx, key, o)
		if err != nil {
			return nil, err
		}
		if ok {
			return held, nil
		}
		if !sleep(ctx, o.RetryInterval) {
			return nil, ctx.Err()
		}
	}
}

// TryAcquire attempts to take the Lease once without blocking.
func (l *k8sLocker) TryAcquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, bool, error) {
	o := lock.Apply(opts...)
	return l.tryOnce(ctx, key, o)
}

// Close releases backend resources. The Kubernetes clientset holds no
// connection that must be torn down, so this is a no-op kept for interface
// completeness and idempotency.
func (l *k8sLocker) Close() error { return nil }

// tryOnce builds a Lease handle for key and attempts a single acquire-or-renew.
// On success it starts the handle's renewal loop and returns a held Lock; on
// contention it returns ok=false with no error.
func (l *k8sLocker) tryOnce(ctx context.Context, key string, o lock.Options) (lock.Lock, bool, error) {
	h := &k8sLock{
		key:           key,
		token:         o.Token,
		ttl:           o.TTL,
		renewInterval: o.RenewInterval,
		rl: &resourcelock.LeaseLock{
			LeaseMeta:  metav1.ObjectMeta{Namespace: l.namespace, Name: l.keyPrefix + key},
			Client:     l.client.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{Identity: o.Token},
		},
		stop: make(chan struct{}),
		lost: make(chan struct{}),
	}
	ok, err := h.tryAcquireOrRenew(ctx)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	h.start()
	return h, true, nil
}

// k8sLock is the [lock.Lock] handle for a single held Lease. It renews the lease
// in the background and fires Lost() when renewal fails (API unreachable past
// the lease duration, or another holder took over).
type k8sLock struct {
	key           string
	token         string
	ttl           time.Duration
	renewInterval time.Duration
	rl            *resourcelock.LeaseLock

	stop     chan struct{} // closed by Unlock to stop the renew loop
	stopOnce sync.Once
	lost     chan struct{}
	lostOnce sync.Once
}

// Key returns the caller-facing key (without KeyPrefix).
func (h *k8sLock) Key() string { return h.key }

// Token returns the holder identity written into the Lease's holderIdentity —
// the fencing token from lock.Options (or the random one from lock.Apply).
func (h *k8sLock) Token() string { return h.token }

// Lost is closed when the lease can no longer be renewed or has been taken over,
// or when Unlock has been called. A long critical section should select on it.
func (h *k8sLock) Lost() <-chan struct{} { return h.lost }

// Unlock stops renewal and best-effort releases the Lease by clearing its
// holderIdentity so a waiter can take it immediately instead of waiting out the
// lease duration. It is idempotent.
func (h *k8sLock) Unlock(ctx context.Context) error {
	h.stopOnce.Do(func() { close(h.stop) })
	h.fireLost()

	// Best-effort release: clear holderIdentity. A failure here (lease already
	// lost, API blip) is harmless — the lease expires after its duration
	// regardless — so it is not surfaced, keeping Unlock idempotent per the
	// abstraction's contract.
	record, _, err := h.rl.Get(ctx)
	if err != nil {
		return nil
	}
	if record.HolderIdentity != h.token {
		return nil // someone else already owns it; nothing to release
	}
	empty := *record
	empty.HolderIdentity = ""
	empty.LeaseDurationSeconds = 1
	empty.RenewTime = metav1.NewTime(time.Unix(0, 0))
	_ = h.rl.Update(ctx, empty)
	return nil
}

// start launches the background renewal loop for a held lease.
func (h *k8sLock) start() { go h.renewLoop() }

// renewLoop refreshes the lease every renewInterval. It fires Lost() when a
// renewal proves the lease is gone (taken over by another holder) or when the
// API has been unreachable for longer than the lease duration, so a holder that
// silently lost its lease stops its critical section.
func (h *k8sLock) renewLoop() {
	t := time.NewTicker(h.renewInterval)
	defer t.Stop()
	lastRenew := time.Now()
	for {
		select {
		case <-h.stop:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), h.renewInterval)
			ok, err := h.tryAcquireOrRenew(ctx)
			cancel()
			switch {
			case ok && err == nil:
				lastRenew = time.Now()
			case !ok && err == nil:
				// Lease taken over by another holder: leadership is lost now.
				h.fireLost()
				return
			default:
				// Transient API error: tolerate until the lease would have
				// expired, then declare the lock lost.
				if time.Since(lastRenew) >= h.ttl {
					h.fireLost()
					return
				}
			}
		}
	}
}

func (h *k8sLock) fireLost() {
	h.lostOnce.Do(func() { close(h.lost) })
}

// tryAcquireOrRenew performs one acquire-or-renew against the Lease, returning
// (true, nil) when this instance now holds the lease, (false, nil) when it is
// validly held by another holder, and (false, err) on an API failure. The logic
// mirrors client-go's leaderelection: create the Lease if absent, take it if
// expired or already ours, otherwise report contention.
func (h *k8sLock) tryAcquireOrRenew(ctx context.Context) (bool, error) {
	now := metav1.Now()
	desired := resourcelock.LeaderElectionRecord{
		HolderIdentity:       h.token,
		LeaseDurationSeconds: h.leaseSeconds(),
		RenewTime:            now,
		AcquireTime:          now,
	}

	oldRecord, _, err := h.rl.Get(ctx)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return false, errutil.Explain(err, "lock-k8s: get lease %s", h.rl.Describe())
		}
		// No lease yet: create one owned by us.
		if err := h.rl.Create(ctx, desired); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return false, nil // lost the create race; treat as contended
			}
			return false, errutil.Explain(err, "lock-k8s: create lease %s", h.rl.Describe())
		}
		return true, nil
	}

	// A valid lease held by someone else means we lose this round.
	if oldRecord.HolderIdentity != "" &&
		oldRecord.HolderIdentity != h.token &&
		oldRecord.RenewTime.Add(h.ttl).After(now.Time) {
		return false, nil
	}

	// Either it is expired, unheld, or already ours: take/renew it. Preserve the
	// acquire time and bump the transition counter only on a real handover.
	if oldRecord.HolderIdentity == h.token {
		desired.AcquireTime = oldRecord.AcquireTime
		desired.LeaderTransitions = oldRecord.LeaderTransitions
	} else {
		desired.LeaderTransitions = oldRecord.LeaderTransitions + 1
	}
	if err := h.rl.Update(ctx, desired); err != nil {
		if apierrors.IsConflict(err) {
			return false, nil // concurrent writer won; treat as contended
		}
		return false, errutil.Explain(err, "lock-k8s: update lease %s", h.rl.Describe())
	}
	return true, nil
}

// leaseSeconds returns the lease duration in whole seconds, clamped to at least
// one, since a Lease's leaseDurationSeconds is an integer field.
func (h *k8sLock) leaseSeconds() int {
	s := int(h.ttl / time.Second)
	if h.ttl%time.Second != 0 {
		s++
	}
	if s < 1 {
		s = 1
	}
	return s
}

// sleep waits for d or until ctx is done; it returns false if ctx ended first.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
