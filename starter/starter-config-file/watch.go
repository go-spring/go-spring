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

package StarterConfigFile

import (
	"github.com/fsnotify/fsnotify"
)

// ensureWatch starts a background directory watcher for dir, deduplicated so
// repeated Load calls (startup + every refresh) do not stack watchers on the
// same mount. Watching is best-effort: if a watcher cannot be created, startup
// still succeeds with a static snapshot, only losing hot-reload for this mount.
func ensureWatch(dir string) {
	watchedMu.Lock()
	if _, ok := watched[dir]; ok {
		watchedMu.Unlock()
		return
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		watchedMu.Unlock()
		return
	}
	if err = w.Add(dir); err != nil {
		_ = w.Close()
		watchedMu.Unlock()
		return
	}
	watched[dir] = struct{}{}
	watchedMu.Unlock()

	go watchLoop(w)
}

// watchLoop drains a watcher's events and triggers a full application property
// refresh on any change. It intentionally reacts to every event rather than
// filtering by file name: a Kubernetes ConfigMap/Secret update surfaces as a
// CREATE/RENAME on the "..data" symlink (not on the individual key files), so
// coalescing every event into one refresh is both correct and simplest. The
// refresh re-runs the provider, which re-reads the mount and propagates new
// values to bound gs.Dync fields.
func watchLoop(w *fsnotify.Watcher) {
	for {
		select {
		case _, ok := <-w.Events:
			if !ok {
				return
			}
			triggerRefresh()
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}
