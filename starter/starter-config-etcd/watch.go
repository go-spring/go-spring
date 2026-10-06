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

package StarterConfigEtcd

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/goutil"
	"go-spring.org/stdlib/randutil"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// watchCore owns the whole watch side of the provider: the watch generation
// context, the dedup set, the watch loops, and the change-to-refresh bridge.
// etcdCtrl embeds it and contributes the client cache and the load path; the
// shared mutex also guards that client cache.
type watchCore struct {
	mu       sync.Mutex
	listened map[string]struct{}

	// ctx is the watch generation: Close cancels it — stopping every watch
	// goroutine — and mints the next generation right away, so a registration
	// after Close always finds a live one
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

// stop cancels the watch generation and clears the dedup set. The controller's
// Close calls it and then closes the clients on its own side.
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
		log.Infof(context.Background(), starterTag, "etcd watchers stopped")
	}
}

// registerWatch installs an etcd change watcher for the given key,
// deduplicated across repeated Load calls. since is the revision the initial
// read observed (0 when unknown): the watch resumes from there so a change
// landing right after that read is delivered rather than skipped. optional
// records whether the import declared the key optional: deleting an optional
// key is an expected transition (its properties simply disappear), while
// deleting a required one leaves the last snapshot in place, which the watcher
// surfaces as a warning.
func (w *watchCore) registerWatch(cli etcdClient, cs configSource, optional bool, since int64) {
	lk := clientKey(cs) + "|" + cs.key

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
	// lets the loop's own lines name the key without interpolating it.
	ctx = log.WithFields(ctx, sourceFields(cs)...)

	log.Infof(ctx, starterTag, "watching etcd key for changes")

	goutil.Go(ctx, func(ctx context.Context) {
		defer wg.Done()
		watchLoop(ctx, cli, cs, optional, since)
	}, goutil.InheritCancel)
}

// sourceFields names the key a watch generation is bound to. The same tuple
// goes on the generation's context (the loop's own lines) and on the root each
// change event mints for itself.
func sourceFields(cs configSource) []log.Field {
	return []log.Field{
		log.String("endpoint", cs.endpoint),
		log.String("key", cs.key),
	}
}

// failLogInterval is the alarm cadence: a sustained failure re-warns about this
// often. The loop turns it into a failure count instead of reading the clock
// every retry.
const failLogInterval = 5 * time.Minute

// watchLoop runs the subscribe loop for a single key. It resubscribes whenever
// the channel closes, resuming from the last revision acted on so a change
// during the gap is replayed rather than lost, and it throttles the failure
// alarm: a failure warns, a sustained one re-warns every failLogEvery
// consecutive failures, and recovery logs once.
func watchLoop(wctx context.Context, cli etcdClient, cs configSource, optional bool, since int64) {
	// failLogEvery is how many failure events pass between alarm logs, derived
	// from the alarm cadence and the retry cadence so a sustained failure
	// re-warns about every failLogInterval. Clamped to at least 1, so a retry
	// interval longer than the cadence (or a non-positive one) still logs every
	// failure.
	failLogEvery := 1
	if retry := time.Duration(cs.retryMs) * time.Millisecond; retry > 0 {
		failLogEvery = max(1, int(failLogInterval/retry))
	}
	// failures counts failure events since the last healthy response; the alarm
	// fires on the first and then every failLogEvery-th. A healthy response
	// zeroes it, so the next failure warns immediately.
	failures := 0

	// resume is the revision to watch on from next; 0 means "from the
	// current one". It starts at the revision the initial read observed so
	// the first subscription replays a change that landed right after that
	// read, and it advances to every change acted on so a resubscription
	// after an outage replays whatever happened in the gap instead of
	// losing it (a subscription with no revision starts at the current one
	// and skips the gap).
	resume := since

	// fire stamps the trigger with the change's identity (which key, which
	// etcd revision) so the refresh records logged and metered by
	// observability.RefreshConf carry what this round is about. A generation
	// canceled before the trigger means the application is shutting down; drop
	// the change instead of stalling Close's join with a pointless refresh.
	fire := func(rev int64) {
		if wctx.Err() != nil {
			return
		}
		ctx := log.RootFields(append(sourceFields(cs),
			log.String("trace_id", randutil.Hex(16)),
			log.Int("mod_revision", rev))...)
		log.Info(ctx, starterTag, log.Msg("etcd key changed, triggering refresh"))

		_ = observability.RefreshConf(ctx, func(ctx context.Context) error {
			return gs.RefreshProperties(ctx)
		})
	}

	for {
		// WithCreatedNotify makes etcd ack every established watch; that ack
		// is the healthy response that clears the failure state. Without it
		// an idle key would see no response at all, so a recovered watch
		// would stay marked as failing.
		opts := []clientv3.OpOption{clientv3.WithCreatedNotify()}
		if resume > 0 {
			opts = append(opts, clientv3.WithRev(resume+1))
		}
		ch := cli.Watch(wctx, cs.key, opts...)
		for wr := range ch {
			if wr.CompactRevision != 0 {
				// The revision we asked to resume from has been compacted
				// away, so the events in between are gone. Fall back to the
				// current revision and refresh once to resync whatever the
				// key holds now.
				resume = 0
				fire(wr.CompactRevision)
				continue
			}
			if err := wr.Err(); err != nil {
				if failures%failLogEvery == 0 {
					log.Warn(wctx, starterTag,
						log.Err(err),
						log.Msg("etcd watch returned an error"))
				}
				failures++
			} else if failures > 0 {
				log.Infof(wctx, starterTag, "etcd watch recovered")
				failures = 0
			}
			deleted := false
			for _, ev := range wr.Events {
				if ev.Type == clientv3.EventTypeDelete {
					deleted = true
				}
			}
			if deleted && !optional {
				log.Warnf(wctx, starterTag, "etcd key deleted; stale snapshot retained until the key is restored")
			}
			if len(wr.Events) > 0 {
				rev := resume
				if ev := wr.Events[len(wr.Events)-1]; ev.Kv != nil {
					rev = ev.Kv.ModRevision
				}
				resume = rev
				fire(rev)
			}
		}
		// The channel closes either because the application is shutting
		// down (Close cancelled the watch generation) or because the
		// watcher is genuinely dead (unrecoverable error). The former
		// exits; the latter would otherwise exit silently and this key
		// would never refresh again, so resubscribe and keep watching.
		select {
		case <-wctx.Done():
			return
		default:
		}
		if failures%failLogEvery == 0 {
			err := errutil.Explain(nil, "etcd watch channel closed")
			log.Warn(wctx, starterTag,
				log.Err(err),
				log.Int("retry_ms", cs.retryMs),
				log.Msg("etcd watch channel closed, resubscribing"))
		}
		failures++
		select {
		case <-wctx.Done():
			return
		case <-time.After(time.Duration(cs.retryMs) * time.Millisecond):
		}
	}
}
