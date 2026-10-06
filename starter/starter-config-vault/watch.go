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

package StarterConfigVault

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/goutil"
	"go-spring.org/stdlib/randutil"
)

// watchCore owns the whole watch side of the provider: the poll generation
// context, the dedup set, the loaded-fingerprint baseline, the polling loops,
// and the change-to-refresh bridge. vaultCtrl embeds it and contributes the
// client cache and the load path; the shared mutex also guards that client
// cache.
type watchCore struct {
	mu       sync.Mutex
	listened map[string]struct{}

	// loadedFP holds the fingerprint of the currently loaded data, keyed by
	// watch key. Only Load moves it forward; watchLoop reads it as the
	// comparison baseline.
	loadedFP map[string]string

	// ctx is the poll generation: Close cancels it — stopping every poll
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
		loadedFP: map[string]string{},
		ctx:      ctx,
		cancel:   cancel,
		wg:       &sync.WaitGroup{},
	}
}

// stop cancels the poll generation and clears the dedup set.
func (w *watchCore) Close() {
	w.mu.Lock()
	w.cancel()
	w.ctx, w.cancel = context.WithCancel(context.Background())
	wg := w.wg
	w.wg = &sync.WaitGroup{}
	n := len(w.listened)
	w.listened = map[string]struct{}{}
	w.loadedFP = map[string]string{}
	w.mu.Unlock()

	wg.Wait()
	if n > 0 {
		log.Infof(context.Background(), starterTag, "vault watchers stopped")
	}
}

// watchKey identifies a watched secret.
func watchKey(cs configSource) string {
	return clientKey(cs) + "|" + cs.mount + "|" + cs.path
}

// updateBaseline records the fingerprint of the data a Load actually loaded.
func (w *watchCore) updateBaseline(cs configSource, fp string) {
	w.mu.Lock()
	w.loadedFP[watchKey(cs)] = fp
	w.mu.Unlock()
}

// readFunc fetches the raw secret data: the Load path parses whatever it
// returns, and the poll loop compares its fingerprint. Load hands its own
// implementation in (see vaultCtrl.readSecret) so both sides observe the same
// backend.
type readFunc func(ctx context.Context, cli vaultClient, cs configSource) (map[string]any, error)

// registerWatch spawns a background goroutine that polls the secret. read is
// the secret fetch the load path also uses, handed over so the poll loop and
// Load observe the same backend.
func (w *watchCore) registerWatch(cli vaultClient, cs configSource, read readFunc) {
	lk := watchKey(cs)

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
	// lets the loop's own lines name the secret without interpolating it.
	ctx = log.WithFields(ctx, sourceFields(cs)...)

	log.Infof(ctx, starterTag, "watching vault secret for changes")

	goutil.Go(ctx, func(ctx context.Context) {
		defer wg.Done()
		watchLoop(ctx, cli, cs, read, func() string {
			w.mu.Lock()
			defer w.mu.Unlock()
			return w.loadedFP[lk]
		})
	}, goutil.InheritCancel)
}

// sourceFields names the secret a watch generation is bound to. The same tuple
// goes on the generation's context (the loop's own lines) and on the root each
// change event mints for itself.
func sourceFields(cs configSource) []log.Field {
	return []log.Field{
		log.String("address", cs.address),
		log.String("mount", cs.mount),
		log.String("path", cs.path),
	}
}

// failLogInterval is the alarm cadence: a sustained failure re-warns about this
// often. The loop turns it into a failure count instead of reading the clock
// every poll.
const failLogInterval = 5 * time.Minute

// watchLoop polls the secret and triggers a refresh whenever the content
// fingerprint differs from the last loaded value.
//
// The fingerprint is deliberately NOT updated here: loadedFP is the
// "currently loaded" baseline, and only Load moves it forward (Load re-runs
// as part of the RefreshProperties cycle and stamps the new fingerprint once
// the data is actually loaded). Leaving the stale value behind is what makes
// a failed refresh retry on the next poll instead of silently swallowing the
// change.
//
// Read failures are self-healing (the loop keeps polling) but never silent:
// a failure logs a warning, then one warning every failLogEvery consecutive
// failures, and recovery logs once at info.
func watchLoop(ctx context.Context, cli vaultClient, cs configSource,
	read readFunc, baseline func() string) {
	interval := time.Duration(cs.pollMs) * time.Millisecond
	// failLogEvery is how many consecutive failures pass between alarm logs,
	// derived from the alarm cadence and the poll cadence so a sustained
	// failure re-warns about every failLogInterval. Clamped to at least 1, so a
	// poll interval longer than the cadence (or a non-positive one) still logs
	// every failure.
	failLogEvery := 1
	if interval > 0 {
		failLogEvery = max(1, int(failLogInterval/interval))
	}
	// failures counts consecutive failures since the last success; the alarm
	// fires on the first and then every failLogEvery-th. A success zeroes it,
	// so the next failure warns immediately.
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
		data, err := read(ctx, cli, cs)
		if err != nil {
			if failures%failLogEvery == 0 {
				log.Warn(ctx, starterTag,
					log.Err(err),
					log.Msg("vault poll failing, changes are missed until it recovers"))
			}
			failures++
			continue
		}
		if failures > 0 {
			log.Infof(ctx, starterTag, "vault poll recovered")
			failures = 0
		}
		fp := baseline()
		if fpNew := fingerprint(data); fpNew != fp {
			// Stamp the trigger with the change's identity (which secret, new
			// content fingerprint) so the refresh records logged and metered by
			// observability.RefreshConf carry what this round is about. Vault
			// hands the poller no version metadata, so the fingerprint is
			// exactly the value that differs from the loaded baseline this round
			// is refreshing to.
			// A generation canceled before this trigger means the application
			// is shutting down; drop the change instead of stalling Close's
			// join with a pointless refresh.
			if ctx.Err() != nil {
				return
			}
			rctx := log.RootFields(append(sourceFields(cs),
				log.String("trace_id", randutil.Hex(16)),
				log.String("fingerprint", fpNew[:8]))...)
			log.Info(rctx, starterTag, log.Msg("vault secret changed, triggering refresh"))
			_ = observability.RefreshConf(rctx, func(ctx context.Context) error {
				return gs.RefreshProperties(ctx)
			})
		}
	}
}

// fingerprint produces a stable, fixed-length digest of a KV data map for
// change detection. encoding/json sorts map keys, so the marshaled form is
// deterministic regardless of map iteration order; the SHA-256 digest keeps
// the stored fingerprint constant-size (and avoids holding a second plain
// copy of the secret in memory) even for large payloads.
//
// The digest is always 64 hex characters — including for a nil map (which
// marshals to "null") — so the refresh id can safely truncate it. A missing
// secret must still fingerprint distinctly from any present one, which the
// "null" encoding guarantees.
func fingerprint(data map[string]any) string {
	b, err := json.Marshal(data)
	if err != nil {
		// Unreachable for JSON-decoded secret data, but keep the digest
		// fixed-length so a truncating caller stays safe.
		b = []byte("fingerprint error")
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
