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

package lock

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/timeutil"
)

// ElectionConfig configures an [Election]. Locker and Key are required.
type ElectionConfig struct {
	// Locker is the backend used to campaign. Any [Locker] works, so the same
	// election code runs over Redis, etcd, consul or the in-memory locker.
	Locker Locker

	// Key is the shared lock key every candidate contends for. The candidate that
	// holds it is the leader.
	Key string

	// TTL, RenewInterval and RetryInterval tune the underlying acquisition; zero
	// values fall back to the [Options] defaults. RetryInterval also paces how
	// often a follower re-campaigns after losing.
	TTL           time.Duration
	RenewInterval time.Duration
	RetryInterval time.Duration

	// OnStartedLeading is called when this instance becomes leader. Its ctx is cancelled
	// when leadership is lost (lease expired) or the election stops, so a leader's
	// work should honour ctx and return promptly. It runs on its own goroutine;
	// [Election.Run] does not wait for it to return before re-campaigning after a
	// loss, but it is cancelled first.
	OnStartedLeading func(ctx context.Context)

	// OnStoppedLeading is called once each time this instance stops being leader
	// (loss or shutdown). Optional.
	OnStoppedLeading func()
}

// Election runs leader election on top of a [Locker]: many instances call
// [Election.Run], exactly one holds the lock and is the leader at any time, and
// leadership fails over automatically when the current leader crashes or its
// lease expires.
type Election struct {
	cfg    ElectionConfig
	leader atomic.Bool
}

// NewElection builds an [Election]. It returns an error if Locker or Key is
// unset, since a misconfigured election can never elect anyone.
func NewElection(cfg ElectionConfig) (*Election, error) {
	if cfg.Locker == nil {
		return nil, errutil.Explain(nil, "lock: election requires a Locker")
	}
	if cfg.Key == "" {
		return nil, errutil.Explain(nil, "lock: election requires a Key")
	}
	return &Election{cfg: cfg}, nil
}

// IsLeader reports whether this instance currently holds leadership.
func (e *Election) IsLeader() bool { return e.leader.Load() }

// Run campaigns for leadership until ctx is done, then releases the lock and
// returns ctx.Err(). While running it acquires the lock (blocking as a
// follower), invokes OnStartedLeading on win, watches for loss, and re-campaigns. Run
// blocks; typically it is registered as a background runner in the application
// lifecycle.
func (e *Election) Run(ctx context.Context) error {
	opts := []Option{}
	if e.cfg.TTL > 0 {
		opts = append(opts, WithTTL(e.cfg.TTL))
	}
	if e.cfg.RenewInterval != 0 {
		opts = append(opts, WithRenewInterval(e.cfg.RenewInterval))
	}
	if e.cfg.RetryInterval > 0 {
		opts = append(opts, WithRetryInterval(e.cfg.RetryInterval))
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		held, err := e.cfg.Locker.Acquire(ctx, e.cfg.Key, opts...)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Transient backend error: back off and retry the campaign.
			if !timeutil.Sleep(ctx, e.retryInterval()) {
				return ctx.Err()
			}
			continue
		}

		e.serveTerm(ctx, held)

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// serveTerm runs one leadership term: it marks this instance leader, launches
// OnStartedLeading with a term context, and blocks until leadership is lost or ctx ends,
// then resigns and releases the lock.
//
// Leadership transitions are fleet-significant lifecycle events, so each one
// gets its own line on the default tag, log-keyed by lock.key: became leader
// at Info, a lost term at Warn, a term ended by shutdown at Info. A failed
// release is Warned too — the broker may redeliver the key later than expected.
func (e *Election) serveTerm(ctx context.Context, held Lock) {
	termCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	e.leader.Store(true)
	log.Info(ctx, log.TagAppDef,
		log.String("lock.key", e.cfg.Key),
		log.Msg("election: became leader"))

	var wg sync.WaitGroup
	if e.cfg.OnStartedLeading != nil {
		wg.Go(func() {
			e.cfg.OnStartedLeading(termCtx)
		})
	}

	lost := false
	select {
	case <-ctx.Done():
	case <-held.Lost():
		lost = true
	}

	// Resign: stop the leader work, mark follower, run the callback, release.
	e.leader.Store(false)
	cancel()
	wg.Wait()
	if e.cfg.OnStoppedLeading != nil {
		e.cfg.OnStoppedLeading()
	}
	if lost {
		log.Warn(ctx, log.TagAppDef,
			log.String("lock.key", e.cfg.Key),
			log.Msg("election: leadership lost"))
	} else {
		log.Info(ctx, log.TagAppDef,
			log.String("lock.key", e.cfg.Key),
			log.Msg("election: term ended"))
	}
	if err := held.Unlock(context.WithoutCancel(ctx)); err != nil {
		log.Warn(ctx, log.TagAppDef,
			log.String("lock.key", e.cfg.Key),
			log.Err(err),
			log.Msg("election: release failed"))
	}
}

func (e *Election) retryInterval() time.Duration {
	if e.cfg.RetryInterval > 0 {
		return e.cfg.RetryInterval
	}
	return 100 * time.Millisecond
}
