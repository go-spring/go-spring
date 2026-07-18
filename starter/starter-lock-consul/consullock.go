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

package StarterLockConsul

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/lock"
)

// Consul allows session TTL only in [10s, 86400s]; clamp caller-supplied
// values into this window so a misconfigured TTL cannot make LockOpts reject
// the entire starter at boot.
const (
	minSessionTTL = 10 * time.Second
	maxSessionTTL = 86400 * time.Second

	// tryWaitTime bounds a single non-blocking TryAcquire round-trip. Consul's
	// LockTryOnce still needs a wait window in which to observe contention;
	// small values keep the "try once" latency low while giving the agent time
	// to answer.
	tryWaitTime = 500 * time.Millisecond
)

// consulLocker is the [lock.Locker] backed by a shared *api.Client. It hands
// out per-acquisition *api.Lock handles that each own their own consul session,
// so cancelling one handle never affects another.
type consulLocker struct {
	client    *api.Client
	keyPrefix string
	ttl       time.Duration
	closeOnce sync.Once
}

// newConsulLocker builds a locker plus its api.Client from a bound Config. It
// fails fast when Address is empty (see starter.go), and normalises the TTL
// into consul's accepted range.
func newConsulLocker(c Config) (*consulLocker, error) {
	if c.Address == "" {
		return nil, errutil.Explain(nil, "lock-consul: address is required")
	}

	scheme := c.Scheme
	if scheme == "" {
		scheme = "http"
	}
	if c.TLS.Enabled && scheme == "http" {
		// TLS.Enabled implies https unless the caller overrides scheme, but
		// leave an explicit non-http scheme untouched.
		scheme = "https"
	}

	cfg := &api.Config{
		Address: c.Address,
		Scheme:  scheme,
		Token:   c.Token,
	}
	if c.TLS.Enabled {
		cfg.TLSConfig = api.TLSConfig{
			Address:  c.TLS.Address,
			CAFile:   c.TLS.CAFile,
			CertFile: c.TLS.CertFile,
			KeyFile:  c.TLS.KeyFile,
		}
	}

	cli, err := api.NewClient(cfg)
	if err != nil {
		return nil, errutil.Explain(err, "lock-consul: create client for %s failed", c.Address)
	}

	ttl := c.TTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	if ttl < minSessionTTL {
		ttl = minSessionTTL
	}
	if ttl > maxSessionTTL {
		ttl = maxSessionTTL
	}

	kp := c.KeyPrefix
	if kp == "" {
		kp = "lock/"
	}
	return &consulLocker{
		client:    cli,
		keyPrefix: kp,
		ttl:       ttl,
	}, nil
}

// Acquire blocks until the lock is held, ctx is done, or the backend returns a
// non-retriable error. Contended acquisitions wait inside api.Lock's own
// blocking-query loop; we cancel it by closing stopCh from a ctx watcher.
func (l *consulLocker) Acquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, error) {
	o := lock.Apply(opts...)
	al, err := l.buildLock(key, o, false)
	if err != nil {
		return nil, err
	}

	stopCh, cancel := ctxStopCh(ctx)
	defer cancel()

	leaderCh, err := al.Lock(stopCh)
	if err != nil {
		return nil, errutil.Explain(err, "lock-consul: acquire %s failed", key)
	}
	if leaderCh == nil {
		// Only happens when stopCh fired first; surface the ctx error so the
		// caller can tell contention from cancellation.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errutil.Explain(nil, "lock-consul: acquire %s returned no leader channel", key)
	}
	return newConsulLock(al, l.keyPrefix+key, o.Token, leaderCh), nil
}

// TryAcquire makes a single non-blocking attempt. ok=false with nil error is
// ordinary contention; err is reserved for genuine backend failures.
func (l *consulLocker) TryAcquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, bool, error) {
	o := lock.Apply(opts...)
	al, err := l.buildLock(key, o, true)
	if err != nil {
		return nil, false, err
	}

	stopCh, cancel := ctxStopCh(ctx)
	defer cancel()

	leaderCh, err := al.Lock(stopCh)
	if err != nil {
		// api.ErrLockHeld surfaces on a double-acquire of the same *api.Lock;
		// our helper is fresh each call, so this is effectively backend noise.
		if errors.Is(err, api.ErrLockHeld) {
			return nil, false, nil
		}
		return nil, false, errutil.Explain(err, "lock-consul: try-acquire %s failed", key)
	}
	if leaderCh == nil {
		// LockTryOnce returns a nil channel when the lock is held elsewhere or
		// the ctx-derived stopCh fired first.
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		return nil, false, nil
	}
	return newConsulLock(al, l.keyPrefix+key, o.Token, leaderCh), true, nil
}

// Close is idempotent. The api.Client has no Close method; held handles keep
// their own sessions alive on their own goroutines, so there is nothing to
// tear down here except guarding the once-only contract.
func (l *consulLocker) Close() error {
	l.closeOnce.Do(func() {})
	return nil
}

// buildLock constructs a per-acquisition *api.Lock with the shared client. The
// tryOnce flag switches between blocking and single-shot semantics.
func (l *consulLocker) buildLock(key string, o lock.Options, tryOnce bool) (*api.Lock, error) {
	ttl := o.TTL
	if ttl < minSessionTTL {
		ttl = l.ttl
	}
	if ttl > maxSessionTTL {
		ttl = maxSessionTTL
	}
	sessionTTL := fmt.Sprintf("%ds", int(ttl.Round(time.Second).Seconds()))

	opts := &api.LockOptions{
		Key:          l.keyPrefix + key,
		Value:        []byte(o.Token),
		SessionTTL:   sessionTTL,
		SessionName:  "go-spring/lock",
		LockTryOnce:  tryOnce,
		LockWaitTime: tryWaitTime,
	}
	al, err := l.client.LockOpts(opts)
	if err != nil {
		return nil, errutil.Explain(err, "lock-consul: LockOpts for %s failed", key)
	}
	return al, nil
}

// ctxStopCh translates a context into the <-chan struct{} api.Lock expects. It
// returns a cancel func the caller must invoke on the happy path so the goroutine
// exits promptly even when ctx never fires.
func ctxStopCh(ctx context.Context) (<-chan struct{}, context.CancelFunc) {
	stopCh := make(chan struct{})
	// Use a child context so we can also stop the watcher on Acquire return.
	child, cancel := context.WithCancel(ctx)
	go func() {
		<-child.Done()
		close(stopCh)
	}()
	return stopCh, cancel
}

// consulLock is a currently held lock handle. It wraps a single *api.Lock and
// the leaderCh returned by api.Lock.Lock; both are owned by this handle and
// released once via Unlock.
type consulLock struct {
	al       *api.Lock
	key      string
	token    string
	leaderCh <-chan struct{}
	once     sync.Once
}

func newConsulLock(al *api.Lock, key, token string, leaderCh <-chan struct{}) *consulLock {
	return &consulLock{al: al, key: key, token: token, leaderCh: leaderCh}
}

func (h *consulLock) Key() string           { return h.key }
func (h *consulLock) Token() string         { return h.token }
func (h *consulLock) Lost() <-chan struct{} { return h.leaderCh }

// Unlock releases the lock and destroys its session. It is idempotent: a
// second call returns nil, and api.ErrLockNotHeld is treated as a benign
// "already released" signal rather than surfaced as [lock.ErrNotHeld], because
// consul cannot distinguish "we released it" from "somebody else did".
func (h *consulLock) Unlock(_ context.Context) error {
	var out error
	h.once.Do(func() {
		if err := h.al.Unlock(); err != nil && !errors.Is(err, api.ErrLockNotHeld) {
			out = errutil.Explain(err, "lock-consul: unlock %s failed", h.key)
			// Fall through to Destroy — best-effort session cleanup even on error.
		}
		// Destroy is best-effort: it fails with ErrLockInUse if another party
		// grabbed the key between Unlock and Destroy, which is not our problem.
		_ = h.al.Destroy()
	})
	return out
}
