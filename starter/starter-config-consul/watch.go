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

package StarterConfigConsul

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/goutil"
)

// watchCore owns the whole watch side of the provider: the watch generation
// context, the dedup set, the blocking-query loops, and the change-to-refresh
// bridge. The controller holds it as a field and contributes the client
// cache and the load path, each under its own mutex.
type watchCore struct {
	mu       sync.Mutex
	listened map[string]struct{}

	// ctx is the watch generation: Close cancels it — aborting every blocking
	// query — and mints the next generation right away, so a registration
	// after Close always finds a live one.
	ctx    context.Context
	cancel context.CancelFunc
	// wg tracks this generation's watch goroutines; Close waits on it before
	// returning, so no watcher outlives the Close that stopped it.
	wg *sync.WaitGroup
}

func newWatchCore() watchCore {
	ctx, cancel := context.WithCancel(context.Background())
	return watchCore{
		listened: map[string]struct{}{},
		ctx:      ctx,
		cancel:   cancel,
		wg:       &sync.WaitGroup{},
	}
}

// Close stops every watch goroutine and clears the dedup set.
func (w *watchCore) Close() {
	w.mu.Lock()
	w.cancel()
	w.ctx, w.cancel = context.WithCancel(context.Background())
	wg := w.wg
	w.wg = &sync.WaitGroup{}
	n := len(w.listened)
	w.listened = map[string]struct{}{}
	w.mu.Unlock()

	wg.Wait()
	if n > 0 {
		log.Infof(context.Background(), starterTag, "consul watchers stopped")
	}
}

// registerWatch spawns a background goroutine that runs a Consul blocking
// query against the given KV path. Deduplicated across repeated Load calls.
// since is the KV index the initial read observed (0 when unknown): the watch
// resumes from there so a change landing right after that read is delivered
// rather than folded into the baseline and lost. optional records whether the
// import declared the path optional: deleting an optional path is an expected
// transition (its properties simply disappear), while deleting a required one
// leaves the last snapshot in place, which the watcher surfaces as a warning.
func (w *watchCore) registerWatch(cli consulClient, cs configSource, optional bool, since uint64) {
	lk := clientKey(cs) + "|" + cs.kvPath

	w.mu.Lock()
	if _, ok := w.listened[lk]; ok {
		w.mu.Unlock()
		return
	}
	w.listened[lk] = struct{}{}
	w.wg.Add(1)
	wg := w.wg
	ctx := w.ctx
	w.mu.Unlock()

	// The goroutine outlives this load, so it runs on the watch generation's
	// context, not on the load's. It carries the same fields, which is what
	// lets the loop's own lines name the KV path without interpolating it.
	ctx = log.WithFields(ctx,
		log.String("address", cs.address),
		log.String("kv_path", cs.kvPath),
		log.String("datacenter", cs.datacenter))

	log.Infof(ctx, starterTag, "watching consul kv for changes")

	goutil.Go(ctx, func(ctx context.Context) {
		defer wg.Done()
		watchLoop(ctx, cli, cs, optional, since)
	}, goutil.InheritCancel)
}

// failLogInterval is the alarm cadence: a sustained failure re-warns about this
// often. The loop turns it into a failure count instead of reading the clock
// every retry.
const failLogInterval = 5 * time.Minute

// watchLoop runs the blocking-query loop for a single KV path. Errors retry
// visibly-but-throttled: a failure logs a warning, then one warning every
// failLogEvery consecutive failures, and recovery logs once at info.
func watchLoop(ctx context.Context, cli consulClient, cs configSource, optional bool, since uint64) {
	// Resume from the index the initial read observed: the first query then
	// reports any change after that read instead of folding it into the
	// baseline and losing it. since == 0 means no index was observed (the read
	// failed), so the loop falls back to adopting the first query's index.
	lastIndex := since
	initialized := since > 0
	// failLogEvery is how many consecutive failures pass between alarm logs,
	// derived from the alarm cadence and the retry cadence so a sustained
	// failure re-warns about every failLogInterval. Clamped to at least 1, so a
	// retry interval longer than the cadence (or a non-positive one, meaning
	// retry without waiting) still logs every failure.
	failLogEvery := 1
	if retry := time.Duration(cs.retryMs) * time.Millisecond; retry > 0 {
		failLogEvery = max(1, int(failLogInterval/retry))
	}
	// failures counts consecutive failures since the last success; the alarm
	// fires on the first and then every failLogEvery-th. A success zeroes it,
	// so the next failure warns immediately.
	failures := 0
	for {
		// A cancel can also land between two successful queries (the error
		// paths below check it too, but a quiet poll cycle must not paper
		// over a dead generation), so check before issuing the next one.
		select {
		case <-ctx.Done():
			return
		default:
		}

		// The blocking query carries the watch generation's context, so Close
		// aborts an in-flight wait instead of leaving it parked for WaitTime.
		// On top of it, a client-side deadline of WaitTime plus slack bounds
		// the request: a healthy no-change wait completes inside WaitTime,
		// while a black-holed connection — which never reaches the server's
		// clock, so WaitTime cannot cut it — is cut here and falls into the
		// retry path below.
		q := &api.QueryOptions{
			Datacenter: cs.datacenter,
			WaitIndex:  lastIndex,
			WaitTime:   5 * time.Minute,
		}
		qctx, cancel := context.WithTimeout(ctx, 5*time.Minute+30*time.Second)
		pair, meta, err := cli.Get(cs.kvPath, q.WithContext(qctx))
		cancel()
		if err == nil && meta == nil {
			err = errutil.Explain(nil, "nil query meta")
		}
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			if failures%failLogEvery == 0 {
				log.Warn(ctx, starterTag,
					log.Err(err),
					log.Int("retry_ms", cs.retryMs),
					log.Msg("consul watch failing, changes are missed until it recovers"))
			}
			failures++
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(cs.retryMs) * time.Millisecond):
			}
			continue
		}
		if failures > 0 {
			log.Infof(ctx, starterTag, "consul watch recovered")
			failures = 0
		}
		if meta.LastIndex < lastIndex {
			lastIndex = 0
			continue
		}
		if !initialized {
			lastIndex = meta.LastIndex
			initialized = true
			continue
		}
		if meta.LastIndex > lastIndex {
			lastIndex = meta.LastIndex
			if pair == nil && !optional {
				log.Warnf(ctx, starterTag, "consul kv deleted; stale snapshot retained until the key is restored")
			}
			// Stamp the trigger with the change's identity (which KV entry, which
			// consul revision) so the refresh records logged and metered by
			// observability.RefreshConf carry what this round is about. The LastIndex
			// doubles as the refresh identifier: it is unique per KV change within
			// consul, so two refreshes from the same path are distinguishable.
			// A generation canceled between the query and this trigger means
			// the application is shutting down: the change is real, but no
			// one will consume a refreshed snapshot, and a synchronous
			// refresh here would stall Close's join. Drop it.
			if ctx.Err() != nil {
				return
			}

			refreshID := fmt.Sprintf("consul:%s@%d", cs.kvPath, meta.LastIndex)

			log.Info(ctx, starterTag,
				log.String("refresh_id", refreshID),
				log.Msg("consul kv changed, triggering refresh"))

			_ = observability.RefreshConf(refreshID, func() error {
				return gs.RefreshProperties(refreshID)
			})
		}
	}
}
