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

package source

import (
	"context"
	"os"
	"path/filepath"
	"reflect"

	"github.com/fsnotify/fsnotify"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/randutil"
)

// FileConfig is the file source's configuration, bound under whatever key the
// deployment's source starter chooses: the implementation carries no prefix.
type FileConfig struct {
	// Path is the document to watch. Required.
	Path string `value:"${path}" expr:"$ != ''"`
}

// FileSource is a [Source] backed by ONE standalone rules file — the self-built
// refresh chain. It watches the file with fsnotify and pushes each good
// re-parse to its subscriber; the root's center then fans the new document out
// to its authorities. It deliberately bypasses gs's properties-refresh pipeline:
// a rule change refreshes that root only, never re-binds the whole app.
//
// What the file contains is decided by T: [Parse] binds it through the same
// value-tag machinery every root uses, so the transport is implemented once and
// instantiated per root (`NewFileSource[admission.Settings]`,
// `NewFileSource[kfault.Document]`, ...).
//
// A parse failure on reload keeps the last good snapshot and logs — the Source
// contract is "everything pushed is vouched for"; a bad edit must not blank a
// live root. A failure on the INITIAL load fails construction, so a
// misconfigured path surfaces at startup instead of leaving the center on its
// empty document.
type FileSource[T any] struct {
	path string

	// PushSource is embedded BY VALUE so this transport inherits the [Source]
	// read and subscribe pair (the fsnotify loop feeds it through reload). A
	// value embed needs no separate allocation and its zero value is a valid
	// empty document, so a constructor mistake cannot leave a nil embed behind.
	// Embedding also promotes Push — the INTERNAL commit path that skips
	// reload's parse and change-dedup guards — so callers must never call Push
	// directly.
	PushSource[T]

	// watcher and stopped are touched only by Run and Close, with no lock: gs
	// invokes Run once after wiring and Close once at shutdown, in that order
	// (the watchLoop goroutine reads stopped, an immutable channel, and
	// synchronizes through its close, not a field). Close additionally tolerates
	// a source that was never started — a caller may build one, read its
	// snapshot and drop it without watching.
	watcher *fsnotify.Watcher
	stopped chan struct{}
}

// NewFileSource loads path once through [Parse] and returns a FileSource holding
// that snapshot. The watcher starts in [FileSource.Run] (the gs Runner hook); a
// standalone caller without a container calls the same method directly.
func NewFileSource[T any](path string) (*FileSource[T], error) {
	src := &FileSource[T]{path: path, stopped: make(chan struct{})}
	cfg, err := src.load()
	if err != nil {
		return nil, err
	}
	src.PushSource = PushSource[T]{cfg: cfg}
	return src, nil
}

// Run is the gs Runner hook: begin watching the rules file. The loop
// deliberately begins here — after every bean (the centers above all) is wired
// and armed — so no delivery can race construction. Run returns immediately;
// the watcher runs independently of whether a center ever reads a snapshot.
func (s *FileSource[T]) Run(_ context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return errutil.Explain(err, "controlplane file source: create watcher failed")
	}
	// Watch the parent directory, never the file itself: Kubernetes updates a
	// ConfigMap/Secret mount by atomically renaming the "..data" symlink, and
	// common editors save by atomic rename too — in both cases the file's inode
	// changes and a per-file watch would go stale after the first swap.
	if err = w.Add(filepath.Dir(s.path)); err != nil {
		_ = w.Close()
		return errutil.Explain(err, "controlplane file source: watch %s failed", filepath.Dir(s.path))
	}
	s.watcher = w

	// The source's own identity rides on its loop context: the lines below name
	// the file without repeating it.
	go s.watchLoop(log.RootFields(log.String("path", s.path)), w)
	return nil
}

// Close stops the watcher. It is idempotent: the bean's Destroy hook calls it,
// and a standalone caller may close the source itself without making that hook
// panic on a second close.
func (s *FileSource[T]) Close() error {
	if s.watcher == nil { // never started, or already closed
		return nil
	}
	close(s.stopped)
	w := s.watcher
	s.watcher = nil
	return w.Close()
}

// watchLoop coalesces directory events into reloads. Reacting to every event
// (not just the rules file's name) is deliberate: an atomic-rename update
// surfaces as events on temp names or the "..data" symlink, not on the file.
func (s *FileSource[T]) watchLoop(ctx context.Context, w *fsnotify.Watcher) {
	for {
		select {
		case <-s.stopped:
			return
		case _, ok := <-w.Events:
			if !ok {
				return
			}
			s.reload(ctx)
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

// reload re-reads and re-parses the file; on success it pushes the new config
// (skipping no-op re-deliveries of an identical one). On failure it keeps the
// last good snapshot and logs.
func (s *FileSource[T]) reload(ctx context.Context) {
	cfg, err := s.load()
	if err != nil {
		log.Error(ctx, sourceTag, err, "controlplane file source: reload failed (keeping last good config)")
		return
	}
	if reflect.DeepEqual(s.Snapshot(), cfg) {
		return // a touch that did not change the document pushes nothing
	}
	// A changed document starts a path here and continues into the center that
	// applies it: the change mints the trace_id both sides log against.
	s.Push(log.RootFields(
		log.String("trace_id", randutil.Hex(16)),
		log.String("path", s.path)), cfg)
}

// load reads and parses the document through [Parse] — the same parse every
// adapter for this root uses, so a rules file is backend-portable byte for byte.
func (s *FileSource[T]) load() (T, error) {
	var doc T
	data, err := os.ReadFile(s.path)
	if err != nil {
		return doc, errutil.Explain(err, "controlplane file source: read %s failed", s.path)
	}
	return Parse[T](s.path, data, "")
}
