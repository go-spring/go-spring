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

package StarterGovernance

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/fsnotify/fsnotify"
	"go-spring.org/cloud/governance"
	"go-spring.org/log"
	"go-spring.org/starter-governance/rules"
	"go-spring.org/stdlib/errutil"
)

// FileSource is a governance.Source backed by ONE standalone rules file — the
// self-built refresh chain. It watches the file with fsnotify and pushes each
// good re-parse to its subscriber; the governance center then fans the new
// policy out to every registered resource. It deliberately
// bypasses gs's properties-refresh pipeline: a governance rule change refreshes
// governance only, never re-binds the whole app.
//
// The file uses the SAME keys an app.properties entry would (govern.enabled,
// govern.default.attempt-timeout, govern.rules[0].resources, ...), parsed by
// extension through the shared conf reader registry (json/properties/yaml/
// toml), so a rules file is literally a snippet of app.config carved out.
//
// A parse failure on reload keeps the last good snapshot and logs — the Source
// contract is "everything pushed is vouched for"; a bad edit must not blank
// live governance. A failure on the INITIAL load fails construction, so a
// misconfigured path surfaces at startup instead of arming a disabled center.
type FileSource struct {
	path string

	mu  sync.Mutex
	cfg governance.Config
	cb  func(governance.Config)

	watcher *fsnotify.Watcher
	stopped chan struct{}
}

// NewFileSource loads path once and returns a FileSource holding that snapshot.
// The watcher is started by [FileSource.Init] (the gs bean lifecycle); the
// standalone path (no container) starts it with [FileSource.Start].
func NewFileSource(path string) (*FileSource, error) {
	src := &FileSource{path: path, stopped: make(chan struct{})}
	cfg, err := src.load()
	if err != nil {
		return nil, err
	}
	src.cfg = cfg
	return src, nil
}

// Start begins watching the rules file. Idempotent; used by Init so the
// watcher runs whether or not the center ever subscribes (a bean-injected
// source that loses priority to an explicit SetSource still stays warm).
func (s *FileSource) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.watcher != nil {
		return nil
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return errutil.Explain(err, "governance file source: create watcher failed")
	}
	// Watch the parent directory, never the file itself: Kubernetes updates a
	// ConfigMap/Secret mount by atomically renaming the "..data" symlink, and
	// common editors save by atomic rename too — in both cases the file's
	// inode changes and a per-file watch would go stale after the first swap.
	if err = w.Add(filepath.Dir(s.path)); err != nil {
		_ = w.Close()
		return errutil.Explain(err, "governance file source: watch %s failed", filepath.Dir(s.path))
	}
	s.watcher = w

	go s.watchLoop(w)
	return nil
}

// Init is the gs lifecycle hook: start watching.
func (s *FileSource) Init() error { return s.Start() }

// Close stops the watcher. Implements the optional-close contract the
// governance center probes for on Destroy.
func (s *FileSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.watcher == nil {
		return nil
	}
	select {
	case <-s.stopped:
		// already closed
	default:
		close(s.stopped)
	}
	err := s.watcher.Close()
	s.watcher = nil
	return err
}

// watchLoop coalesces directory events into reloads. Reacting to every event
// (not just the rules file's name) is deliberate: an atomic-rename update
// surfaces as events on temp names or the "..data" symlink, not on the file.
func (s *FileSource) watchLoop(w *fsnotify.Watcher) {
	for {
		select {
		case <-s.stopped:
			return
		case _, ok := <-w.Events:
			if !ok {
				return
			}
			s.reload()
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

// Snapshot returns the latest good snapshot.
func (s *FileSource) Snapshot() governance.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Subscribe registers cb as the push target (the center is the only consumer).
func (s *FileSource) Subscribe(cb func(governance.Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cb = cb
}

// reload re-reads and re-parses the file; on success it swaps the snapshot and
// pushes it (skipping no-op re-deliveries of an identical config). On failure
// it keeps the last good snapshot and logs.
func (s *FileSource) reload() {
	cfg, err := s.load()
	if err != nil {
		log.Errorf(context.Background(), starterTag, "governance file source: reload %s failed (keeping last good config): %v", s.path, err)
		return
	}

	s.mu.Lock()
	unchanged := reflect.DeepEqual(s.cfg, cfg)
	s.cfg = cfg
	cb := s.cb
	s.mu.Unlock()

	if unchanged {
		return // a touch that did not change the rules pushes nothing
	}
	if cb != nil {
		cb(cfg)
	}
}

// load reads and parses the rules file into a governance.Config through the
// shared [parseGovernanceDoc] core — the same parse every Source adapter in
// this starter uses, so a rules file is backend-portable byte for byte.
func (s *FileSource) load() (governance.Config, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return governance.Config{}, errutil.Explain(err, "governance file source: read %s failed", s.path)
	}
	return rules.Parse(s.path, data, "")
}
