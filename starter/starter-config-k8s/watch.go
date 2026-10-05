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

package StarterConfigK8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// watchManager tracks informers so they can be stopped on shutdown, and
// deduplicates watchers so repeated Load calls do not stack informers.
type watchManager struct {
	mu      sync.Mutex
	watched map[string]struct{}
	stops   []chan struct{}
}

// forget drops a watcher id so a later Load may retry starting it.
func (m *watchManager) forget(id string) {
	m.mu.Lock()
	delete(m.watched, id)
	m.mu.Unlock()
}

// stopAll stops every running informer.
func (m *watchManager) stopAll() {
	m.mu.Lock()
	stops := m.stops
	m.stops = nil
	m.watched = map[string]struct{}{}
	m.mu.Unlock()
	for _, s := range stops {
		close(s)
	}
	if len(stops) > 0 {
		log.Infof(context.Background(), starterTag, "k8s informers stopped")
	}
}

// objVersion extracts an informer object's resourceVersion for refresh
// tagging. The handlers receive the concrete typed objects, which all
// implement metav1.Object; wrappers like cache.DeletedFinalStateUnknown do
// not, and yield an empty version.
func objVersion(obj any) string {
	if o, ok := obj.(metav1.Object); ok {
		return o.GetResourceVersion()
	}
	return ""
}

// trigger logs the informer event and fires the application-wide property
// refresh. The handler callbacks run on the informer's goroutines with no
// context of their own, so the log line carries the event's identity in full.
// The refresh itself runs on the application's own context; its outcome
// (status, duration, error) is logged and metered centrally by
// observability.RefreshConf.
func (c *k8sCtrl) trigger(id string, obj any, event string) {
	refreshID := fmt.Sprintf("k8s:%s@%s", id, objVersion(obj))
	log.Info(context.Background(), starterTag,
		log.String("refresh_id", refreshID),
		log.String("event", event),
		log.Msg("watched object changed, triggering refresh"))
	if c.onTrigger != nil {
		c.onTrigger()
		return
	}
	_ = observability.RefreshConf(refreshID, func() error {
		return gs.RefreshProperties(refreshID)
	})
}

// watchSyncTimeout bounds how long Load waits for a new informer's initial
// cache sync before giving up on hot-reload for that object.
var watchSyncTimeout = 30 * time.Second

// ensureWatch starts a namespaced, name-scoped informer on the target object
// and triggers a full property refresh on every add/update/delete.
func (c *k8sCtrl) ensureWatch(ctx context.Context, client k8sClient, cs configSource) {
	// The identity includes kubeconfig: the same object name in two clusters is
	// two different watches backed by two different clientsets, so leaving it
	// out would let the second import dedup against the first and never watch.
	id := fmt.Sprintf("%s/%s/%s/%s", cs.kubeconfig, cs.kind, cs.namespace, cs.objectName)

	c.manager.mu.Lock()
	if _, ok := c.manager.watched[id]; ok {
		c.manager.mu.Unlock()
		return
	}
	c.manager.watched[id] = struct{}{}
	c.manager.mu.Unlock()

	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		0, // event-driven only; no periodic resync needed for a single object
		informers.WithNamespace(cs.namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.FieldSelector = "metadata.name=" + cs.objectName
		}),
	)

	var informer cache.SharedIndexInformer
	switch cs.kind {
	case kindConfigMap:
		informer = factory.Core().V1().ConfigMaps().Informer()
	case kindSecret:
		informer = factory.Core().V1().Secrets().Informer()
	default:
		return
	}

	handler := cache.ResourceEventHandlerFuncs{
		// Stamp each trigger with the change's identity (which object, which
		// resourceVersion) so the refresh records logged and metered by
		// observability.RefreshConf carry what this round is about. The
		// resourceVersion doubles as the refresh identifier: it advances on
		// every write to the object, so two refreshes from the same object are
		// distinguishable. UpdateFunc stamps the new object's version.
		AddFunc: func(obj any) {
			c.trigger(id, obj, "added")
		},
		UpdateFunc: func(_, newObj any) {
			c.trigger(id, newObj, "updated")
		},
		DeleteFunc: func(obj any) {
			c.trigger(id, obj, "deleted")
		},
	}
	if _, err := informer.AddEventHandler(handler); err != nil {
		log.Errorf(ctx, starterTag, err, "k8s config: add the event handler failed; it will not hot-reload until a restart")
		c.manager.forget(id)
		return
	}

	stop := make(chan struct{})
	factory.Start(stop)
	// Bound the sync wait: WaitForCacheSync alone would block Load (and thus
	// app startup) forever when the API server keeps failing the initial LIST
	// (e.g. RBAC missing the watch verb) — the reflector retries indefinitely
	// and the cache never syncs.
	synced := make(chan bool, 1)
	go func() { synced <- cache.WaitForCacheSync(stop, informer.HasSynced) }()
	select {
	case ok := <-synced:
		if !ok {
			close(stop)
			err := errutil.Explain(nil, "k8s config watch stopped before cache sync")
			log.Errorf(ctx, starterTag, err, "k8s config: watch stopped before cache sync; it will not hot-reload until a restart")
			c.manager.forget(id)
			return
		}
	case <-time.After(watchSyncTimeout):
		close(stop)
		err := errutil.Explain(nil, "k8s config cache sync timed out after %s", watchSyncTimeout)
		log.Error(ctx, starterTag, err,
			log.String("timeout", watchSyncTimeout.String()),
			log.Msg("k8s config: cache sync timed out; it will not hot-reload until a restart"))
		c.manager.forget(id)
		return
	}

	c.manager.mu.Lock()
	// A Close may have run while the informer was syncing: stopAll cleared the
	// watcher set and stopped everything it knew about. Appending our stop to
	// the fresh slice would leave it never closed, leaking the informer — so
	// detect that and stop ourselves instead.
	if _, ok := c.manager.watched[id]; !ok {
		c.manager.mu.Unlock()
		close(stop)
		return
	}
	c.manager.stops = append(c.manager.stops, stop)
	c.manager.mu.Unlock()

	log.Infof(ctx, starterTag, "watching k8s object for changes")
}
