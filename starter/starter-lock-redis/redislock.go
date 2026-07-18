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

package StarterLockRedis

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/stdlib/lock"
)

// unlockScript releases the key only when it still carries the caller's token
// (compare-and-DEL). It returns:
//
//	1  — key existed with the caller's token and was deleted.
//	0  — key did not exist (already released/expired).
//	-1 — key exists but is owned by a different token (proof of takeover).
//
// The three-way return lets Unlock distinguish "no-op" from "someone else owns
// it now" without a separate GET round-trip.
var unlockScript = redis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
    return redis.call('del', KEYS[1])
end
if redis.call('exists', KEYS[1]) == 1 then
    return -1
end
return 0
`)

// renewScript extends the key's TTL only if the caller still owns it.
// Returns 1 on success, 0 when the key is gone or now owned by someone else —
// which the renew loop treats as "lock lost".
var renewScript = redis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
    return redis.call('pexpire', KEYS[1], ARGV[2])
end
return 0
`)

// redisLocker implements lock.Locker on top of a *redis.Client using the
// SET NX PX / compare-and-DEL / compare-and-PEXPIRE Redlock-single-node
// pattern. Multi-node Redlock is not implemented: the common case is a single
// Redis (single, sentinel-failover, or a single cluster), and callers who need
// stronger guarantees than a single Redis provides should reach for etcd/consul
// backends via a blank-import swap.
type redisLocker struct {
	cfg    Config
	client *redis.Client

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// newRedisLocker builds a locker over an already-constructed *redis.Client.
// The client's lifecycle (Ping, Close, ...) is owned by starter-go-redis;
// this locker never closes it.
func newRedisLocker(c Config, client *redis.Client) (*redisLocker, error) {
	return &redisLocker{
		cfg:    c,
		client: client,
		stop:   make(chan struct{}),
	}, nil
}

// key applies KeyPrefix so multiple apps can safely share a Redis instance.
func (l *redisLocker) key(k string) string { return l.cfg.KeyPrefix + k }

// applyDefaults folds the starter-level defaults into caller-supplied options
// so both "user gave nothing" and "user gave WithTTL only" see consistent
// values. lock.Apply then normalizes anything still zero to package defaults.
func (l *redisLocker) applyDefaults(opts []lock.Option) []lock.Option {
	var seen struct{ ttl, renew, retry bool }
	for _, fn := range opts {
		var probe lock.Options
		fn(&probe)
		if probe.TTL != 0 {
			seen.ttl = true
		}
		if probe.RenewInterval != 0 {
			seen.renew = true
		}
		if probe.RetryInterval != 0 {
			seen.retry = true
		}
	}
	out := make([]lock.Option, 0, len(opts)+3)
	if !seen.ttl && l.cfg.TTL > 0 {
		out = append(out, lock.WithTTL(l.cfg.TTL))
	}
	if !seen.renew && l.cfg.RenewInterval != 0 {
		out = append(out, lock.WithRenewInterval(l.cfg.RenewInterval))
	}
	if !seen.retry && l.cfg.RetryInterval > 0 {
		out = append(out, lock.WithRetryInterval(l.cfg.RetryInterval))
	}
	out = append(out, opts...)
	return out
}

// TryAcquire attempts a single SET key token NX PX ttl. On success it registers
// a handle and, when auto-renew is enabled, spawns a renewLoop goroutine to
// extend the lease until Unlock or Locker.Close.
func (l *redisLocker) TryAcquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, bool, error) {
	o := lock.Apply(l.applyDefaults(opts)...)
	fullKey := l.key(key)

	ok, err := l.client.SetNX(ctx, fullKey, o.Token, o.TTL).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	h := &redisLock{
		locker: l,
		key:    fullKey,
		token:  o.Token,
		lost:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	if o.RenewInterval > 0 {
		l.wg.Add(1)
		go l.renewLoop(h, o.TTL, o.RenewInterval)
	}
	return h, true, nil
}

// Acquire polls TryAcquire until it succeeds or ctx ends. It matches the
// MemoryLocker contract: contention returns after a bounded sleep, backend
// errors surface immediately.
func (l *redisLocker) Acquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, error) {
	o := lock.Apply(l.applyDefaults(opts)...)
	for {
		held, ok, err := l.TryAcquire(ctx, key, opts...)
		if err != nil {
			return nil, err
		}
		if ok {
			return held, nil
		}
		t := time.NewTimer(o.RetryInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-l.stop:
			t.Stop()
			return nil, context.Canceled
		case <-t.C:
		}
	}
}

// Close signals every renew goroutine to exit and waits for them. It does not
// touch handed-out Lock handles (their Unlock stays idempotent) and does not
// close the underlying *redis.Client — starter-go-redis owns that.
func (l *redisLocker) Close() error {
	l.stopOnce.Do(func() {
		close(l.stop)
	})
	l.wg.Wait()
	return nil
}

// renewLoop refreshes the lease every `every` until the handle is unlocked,
// the locker is closed, or Redis reports the key is gone / owned by someone
// else — at which point the handle's lost channel fires.
func (l *redisLocker) renewLoop(h *redisLock, ttl, every time.Duration) {
	defer l.wg.Done()
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	ttlMillis := ttl.Milliseconds()
	if ttlMillis <= 0 {
		ttlMillis = 1
	}

	for {
		select {
		case <-h.done:
			return
		case <-l.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), every)
			res, err := renewScript.Run(ctx, l.client, []string{h.key}, h.token, ttlMillis).Int64()
			cancel()
			if err != nil {
				// Transient failure: try again on the next tick. Redis is the
				// source of truth for lease expiry, so a missed refresh only
				// hurts if it keeps failing until TTL elapses.
				continue
			}
			if res == 0 {
				// The key vanished or a new owner took over: we no longer hold
				// the lock. Fire Lost() so the critical section can bail.
				h.markLost()
				return
			}
		}
	}
}

// redisLock is a currently-held lock returned by redisLocker.TryAcquire.
// It carries the (prefixed) key, the fencing token, and a `lost` channel
// closed exactly once when the lease is lost or the handle is unlocked.
type redisLock struct {
	locker *redisLocker
	key    string
	token  string

	lost     chan struct{}
	lostOnce sync.Once

	// done is closed by Unlock to stop the renew loop without racing on the
	// public `lost` channel. It is separate from `lost` so callers observing
	// Lost() see it fire only after unlock/loss, never proactively.
	done     chan struct{}
	doneOnce sync.Once

	unlockOnce sync.Once
	unlockErr  error
}

func (h *redisLock) Key() string           { return h.key }
func (h *redisLock) Token() string         { return h.token }
func (h *redisLock) Lost() <-chan struct{} { return h.lost }

func (h *redisLock) markLost() {
	h.lostOnce.Do(func() { close(h.lost) })
	h.stopRenew()
}

func (h *redisLock) stopRenew() {
	h.doneOnce.Do(func() {
		close(h.done)
	})
}

// Unlock runs the compare-and-DEL script exactly once. It is idempotent:
// second and later calls are no-ops. It returns lock.ErrNotHeld only when
// Redis proves another token owns the key (script returned -1).
func (h *redisLock) Unlock(ctx context.Context) error {
	h.unlockOnce.Do(func() {
		// Signal the renew loop to stop before we touch Redis so it does not
		// race a fresh PEXPIRE against our DEL.
		h.stopRenew()

		res, err := unlockScript.Run(ctx, h.locker.client, []string{h.key}, h.token).Int64()
		if err != nil {
			h.unlockErr = err
			// Even on backend error, mark lost so Lost() consumers unblock.
			h.lostOnce.Do(func() { close(h.lost) })
			return
		}
		if res == -1 {
			h.unlockErr = lock.ErrNotHeld
		}
		h.lostOnce.Do(func() { close(h.lost) })
	})
	return h.unlockErr
}

// Ensure interface compliance at compile time.
var (
	_ lock.Locker = (*redisLocker)(nil)
	_ lock.Lock   = (*redisLock)(nil)
)
