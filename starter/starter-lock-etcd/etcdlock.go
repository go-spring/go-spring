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

package StarterLockEtcd

import (
	"context"
	"errors"
	"sync"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/spring/lock"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// etcdLocker implements [lock.Locker] on top of the etcd concurrency package.
// It owns one *clientv3.Client shared across acquisitions; every acquired lock
// runs on its own concurrency.Session so its lease and Lost() channel are
// independent from other holds.
type etcdLocker struct {
	client    *clientv3.Client
	keyPrefix string
	ttlSecs   int

	closeOnce sync.Once
}

// newEtcdLocker builds a *clientv3.Client from c and returns a Locker that
// mints one session per acquisition. It fails fast when the cluster is
// unreachable within DialTimeout so a misconfigured application never boots
// with a silently broken lock backend.
func newEtcdLocker(c Config) (*etcdLocker, error) {
	if len(c.Endpoints) == 0 {
		return nil, errutil.Explain(nil, "lock-etcd: endpoints is required")
	}

	tlsCfg, err := c.TLS.Build()
	if err != nil {
		return nil, errutil.Explain(err, "lock-etcd: build TLS")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Endpoints,
		Username:    c.Username,
		Password:    c.Password,
		DialTimeout: c.DialTimeout,
		TLS:         tlsCfg,
	})
	if err != nil {
		return nil, errutil.Explain(err, "lock-etcd: failed to create etcd client")
	}

	// Fail-fast readiness probe: a Status against the first endpoint proves
	// the credentials and TLS material work, so a bad configuration surfaces
	// at boot instead of on the first Acquire.
	ctx, cancel := context.WithTimeout(context.Background(), c.DialTimeout)
	defer cancel()
	if _, err := cli.Status(ctx, c.Endpoints[0]); err != nil {
		_ = cli.Close()
		return nil, errutil.Explain(err, "lock-etcd: startup probe failed for %s", c.Endpoints[0])
	}

	return &etcdLocker{
		client:    cli,
		keyPrefix: c.KeyPrefix,
		ttlSecs:   c.ttlSeconds(),
	}, nil
}

// Acquire blocks until the lock for key is held or ctx is done. It opens a
// fresh session so this hold's lease and Lost() channel are isolated from any
// other outstanding hold.
func (l *etcdLocker) Acquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, error) {
	o := lock.Apply(opts...)
	sess, mu, err := l.newMutex(key)
	if err != nil {
		return nil, err
	}
	if err := mu.Lock(ctx); err != nil {
		_ = sess.Close()
		return nil, err
	}
	return newEtcdLock(key, o.Token, sess, mu), nil
}

// TryAcquire attempts to take the lock once without blocking. concurrency.Mutex
// signals contention with a sentinel ErrLocked, which is translated to ok=false
// so callers can distinguish it from a real backend error.
func (l *etcdLocker) TryAcquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, bool, error) {
	o := lock.Apply(opts...)
	sess, mu, err := l.newMutex(key)
	if err != nil {
		return nil, false, err
	}
	if err := mu.TryLock(ctx); err != nil {
		_ = sess.Close()
		if errors.Is(err, concurrency.ErrLocked) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return newEtcdLock(key, o.Token, sess, mu), true, nil
}

// Close releases the shared etcd client. Locks already handed out own their
// own sessions and remain valid until the caller Unlocks them (or the process
// dies and the leases expire). Close is idempotent.
func (l *etcdLocker) Close() error {
	var err error
	l.closeOnce.Do(func() {
		err = l.client.Close()
	})
	return err
}

// newMutex opens a fresh concurrency.Session (its own lease with automatic
// keepalive) and a Mutex bound to the prefixed key.
func (l *etcdLocker) newMutex(key string) (*concurrency.Session, *concurrency.Mutex, error) {
	sess, err := concurrency.NewSession(l.client, concurrency.WithTTL(l.ttlSecs))
	if err != nil {
		return nil, nil, errutil.Explain(err, "lock-etcd: failed to create session")
	}
	mu := concurrency.NewMutex(sess, l.keyPrefix+key)
	return sess, mu, nil
}

// etcdLock is the [lock.Lock] handle for a single acquisition. It closes its
// session on Unlock, which releases the underlying lease and fires Lost().
type etcdLock struct {
	key     string
	token   string
	session *concurrency.Session
	mutex   *concurrency.Mutex

	once     sync.Once
	released chan struct{} // closed by Unlock
	lost     chan struct{} // fan-in of session.Done and released
}

func newEtcdLock(key, token string, sess *concurrency.Session, mu *concurrency.Mutex) *etcdLock {
	h := &etcdLock{
		key:      key,
		token:    token,
		session:  sess,
		mutex:    mu,
		released: make(chan struct{}),
		lost:     make(chan struct{}),
	}
	// One goroutine per hold merges the two loss signals (session end or
	// voluntary Unlock) into the single channel returned by Lost(). It exits
	// as soon as either signal fires, so no goroutine leaks past the lock's
	// lifetime.
	go func() {
		select {
		case <-sess.Done():
		case <-h.released:
		}
		close(h.lost)
	}()
	return h
}

// Key returns the caller-facing key (without KeyPrefix), matching what was
// passed to Acquire / TryAcquire.
func (h *etcdLock) Key() string { return h.key }

// Token returns the fencing token drawn from lock.Options; etcd concurrency
// does not expose a native fencing token, so the caller's token (or the
// random one supplied by lock.Apply) is reflected here.
func (h *etcdLock) Token() string { return h.token }

// Lost is closed when either the underlying etcd session ends (lease
// expired / connectivity gone) or Unlock has been called. Callers watching a
// long-running critical section should select on it and abort their work.
func (h *etcdLock) Lost() <-chan struct{} { return h.lost }

// Unlock releases the lock and closes its session. It is idempotent: a second
// call returns nil. When the mutex has already been lost (lease expired), the
// error from etcd is discarded so callers can treat unlock-after-loss as a
// no-op, matching the abstraction's contract.
func (h *etcdLock) Unlock(ctx context.Context) error {
	var err error
	h.once.Do(func() {
		// Best-effort mutex.Unlock: if the lease is already gone the delete
		// call may fail, but the session close below still frees resources.
		unlockErr := h.mutex.Unlock(ctx)
		if closeErr := h.session.Close(); closeErr != nil && unlockErr == nil {
			unlockErr = closeErr
		}
		close(h.released)
		// Swallow lease-expiry unlock errors: the abstraction treats an
		// already-released lock as a successful (idempotent) unlock. Only
		// surface errors that are not the natural consequence of a lost
		// lease.
		if unlockErr != nil && !errors.Is(unlockErr, context.Canceled) {
			// A lost lease surfaces via session.Done(); if the caller passed
			// a live ctx, propagate the underlying error so operators can
			// diagnose backend issues.
			err = unlockErr
		}
	})
	return err
}
