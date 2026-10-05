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

package StarterConfigNacos

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/goutil"
)

// watchCore owns the whole watch side of the provider: the listener
// generation context, the dedup set, and the change-to-refresh bridge.
// nacosCtrl embeds it and contributes the client cache and the load path;
// the shared mutex also guards that client cache. Nacos has no watch loop of
// its own: the SDK invokes the installed OnChange callback, so registration
// is the only machinery here.
type watchCore struct {
	mu       sync.Mutex
	listened map[string]struct{}

	// ctx is the listener generation: Close cancels it (and closes every
	// client, which stops the change callbacks) and mints the next generation
	// right away, so a registration after Close always finds a live one. The
	// callbacks carry the context and hand it to the refresh.
	ctx    context.Context
	cancel context.CancelFunc
	// wg tracks this generation's install-retry goroutines; Close waits on it
	// before returning, so no retry outlives the Close that stopped it.
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

// stop cancels the listener generation and clears the dedup set.
func (w *watchCore) Close() {
	w.mu.Lock()
	w.cancel()
	w.ctx, w.cancel = context.WithCancel(context.Background())
	wg := w.wg
	w.wg = &sync.WaitGroup{}
	n := len(w.listened)
	w.listened = map[string]struct{}{}
	w.mu.Unlock()

	// Every retry loop carries the generation's context, so a canceled loop
	// exits promptly; the wait is bounded by whatever in-flight call it is
	// draining.
	wg.Wait()
	if n > 0 {
		log.Infof(context.Background(), starterTag, "nacos listeners stopped")
	}
}

// failLogInterval is the alarm cadence: a sustained failure re-warns about this
// often. The retry loop turns it into a failure count instead of reading the
// clock every retry.
const failLogInterval = 5 * time.Minute

// registerWatch installs the change listener for the source, deduplicated across
// repeated Load calls. The listener outlives this load, so it runs on the watch
// generation's context, not on the load's, and carries the source's identity
// fields — that is what lets its trigger name the data id without listing it
// again.
//
// The first install is synchronous, so the listener is in place before Load
// reads the data id (a change landing right after that read must not be
// missed). A failure then keeps retrying in the background, because a one-shot
// attempt would leave hot-reload off until the next Load — and the only other
// thing that re-runs the import is a refresh triggered elsewhere, which may
// never come.
func (w *watchCore) registerWatch(cli nacosClient, cs configSource) {
	lk := clientKey(cs) + "|" + cs.group + "|" + cs.dataID

	w.mu.Lock()
	if _, ok := w.listened[lk]; ok {
		w.mu.Unlock()
		return
	}
	w.listened[lk] = struct{}{}
	ctx := w.ctx
	w.mu.Unlock()

	ctx = log.WithFields(ctx,
		log.String("server", cs.server),
		log.String("data_id", cs.dataID),
		log.String("group", cs.group))

	if err := listen(ctx, cli, cs); err == nil {
		return
	}

	w.mu.Lock()
	w.wg.Add(1)
	wg := w.wg
	w.mu.Unlock()
	goutil.Go(ctx, func(ctx context.Context) {
		defer wg.Done()
		retryListen(ctx, cli, cs)
	}, goutil.InheritCancel)
}

// listen installs the change listener on the client, firing a full property
// refresh on every change. A canceled generation means the application is
// shutting down: the change is dropped instead of firing a refresh no one will
// consume (on a dead context, no less).
func listen(ctx context.Context, cli nacosClient, cs configSource) error {
	return cli.ListenConfig(vo.ConfigParam{
		DataId: cs.dataID,
		Group:  cs.group,
		OnChange: func(namespace, group, dataId, data string) {
			if ctx.Err() != nil {
				return
			}
			// The refresh id names the group/data id this round is about; nacos
			// hands the listener no change revision, so the pair is the whole
			// native identity available.
			refreshID := fmt.Sprintf("nacos:%s/%s", cs.group, cs.dataID)
			log.Info(ctx, starterTag,
				log.String("refresh_id", refreshID),
				log.Msg("nacos config changed, triggering refresh"))
			_ = observability.RefreshConf(refreshID, func() error {
				return gs.RefreshProperties(refreshID)
			})
		},
	})
}

// retryListen keeps retrying a failed install until it succeeds or the
// generation ends. The alarm is throttled the way the poll loops throttle
// theirs: the first retry warns, a sustained run re-warns every failLogEvery-th,
// and success logs once.
func retryListen(ctx context.Context, cli nacosClient, cs configSource) {
	failLogEvery := 1
	if retry := time.Duration(cs.retryMs) * time.Millisecond; retry > 0 {
		failLogEvery = max(1, int(failLogInterval/retry))
	}
	failures := 0
	for {
		if err := listen(ctx, cli, cs); err == nil {
			log.Infof(ctx, starterTag, "nacos listen recovered")
			return
		} else if failures%failLogEvery == 0 {
			log.Errorf(ctx, starterTag, err, "nacos listen config failed; it will not hot-reload until the listen succeeds")
		}
		failures++
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(cs.retryMs) * time.Millisecond):
		}
	}
}
