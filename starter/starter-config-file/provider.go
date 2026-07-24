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
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register "file-watch" as a configuration provider so that a
	// spring.app.imports entry such as
	//
	//	optional:file-watch:/etc/config?format=properties
	//
	// loads configuration from a mounted directory (or single file) at startup
	// and, whenever the mount changes, triggers a full property refresh.
	//
	// The provider is the global controller's Load method, so the same object
	// that holds the PropertiesRefresher (injected via autowire by the IoC
	// container) also serves config loads — no separate hook wiring needed.
	conf.RegisterProvider("file-watch", fileWatchController.Load)
}

// contentReader parses raw configuration bytes into a nested map based on a
// declared format name (rather than a file extension).
type contentReader func(b []byte) (map[string]any, error)

var contentReaders = map[string]contentReader{
	"properties": prop.Read,
	"props":      prop.Read,
	"yaml":       yaml.Read,
	"yml":        yaml.Read,
	"toml":       toml.Read,
	"tml":        toml.Read,
	"json":       json.Read,
}

// configSource holds the parsed components of a file-watch provider source.
type configSource struct {
	path   string // absolute or relative path to a directory or a single file
	format string // optional format override applied to all files
}

// parseSource parses a provider source of the form <path>[?format=..]. The
// leading "file-watch:" prefix has already been stripped by conf/provider.Load.
func parseSource(source string) (configSource, error) {
	path := source
	var format string
	if p, query, ok := strings.Cut(source, "?"); ok {
		path = p
		q, err := url.ParseQuery(query)
		if err != nil {
			return configSource{}, errutil.Explain(err, "invalid file-watch query in %q", source)
		}
		format = q.Get("format")
	}
	if path == "" {
		return configSource{}, errutil.Explain(nil, "missing path in file-watch source %q", source)
	}
	if format != "" {
		if _, ok := contentReaders[format]; !ok {
			return configSource{}, errutil.Explain(nil, "unsupported file-watch format %q", format)
		}
	}
	return configSource{path: path, format: format}, nil
}

// Load implements conf/provider.Provider. It reads configuration from the
// mounted path, parses it, and installs a directory watcher that triggers an
// application property refresh on change.
func (c *configFileController) Load(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(cs.path)
	if err != nil {
		if os.IsNotExist(err) && optional {
			return nil, nil
		}
		return nil, errutil.Explain(err, "file-watch: stat %s failed", cs.path)
	}

	// Watch the directory (the mount point), not the individual file: Kubernetes
	// updates a ConfigMap/Secret by writing a new timestamped directory and
	// atomically renaming the "..data" symlink, so a per-file watch would be
	// left pointing at a stale inode after the first update.
	watchDir := cs.path
	if !info.IsDir() {
		watchDir = filepath.Dir(cs.path)
	}
	c.ensureWatch(watchDir)

	m := map[string]string{}
	if info.IsDir() {
		if err = c.readDir(cs, m); err != nil {
			return nil, err
		}
	} else {
		if err = c.readOneFile(cs.path, cs.format, m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// --- directory watching ---

// ensureWatch starts a background directory watcher for dir, deduplicated so
// repeated Load calls (startup + every refresh) do not stack watchers on the
// same mount. Watching is best-effort: if a watcher cannot be created, startup
// still succeeds with a static snapshot, only losing hot-reload for this mount.
func (c *configFileController) ensureWatch(dir string) {
	c.mu.Lock()
	if c.watched == nil {
		c.watched = map[string]struct{}{}
	}
	if _, ok := c.watched[dir]; ok {
		c.mu.Unlock()
		return
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		c.mu.Unlock()
		return
	}
	if err = w.Add(dir); err != nil {
		_ = w.Close()
		c.mu.Unlock()
		return
	}
	c.watched[dir] = struct{}{}
	c.mu.Unlock()

	go c.watchLoop(w)
}

// watchLoop drains a watcher's events and triggers a full application property
// refresh on any change. It intentionally reacts to every event rather than
// filtering by file name: a Kubernetes ConfigMap/Secret update surfaces as a
// CREATE/RENAME on the "..data" symlink (not on the individual key files), so
// coalescing every event into one refresh is both correct and simplest.
func (c *configFileController) watchLoop(w *fsnotify.Watcher) {
	for {
		select {
		case _, ok := <-w.Events:
			if !ok {
				return
			}
			c.TriggerRefresh()
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

// --- file reading ---

// readDir merges every eligible file in a mounted directory into m. Entries
// whose name begins with '.' are skipped: this excludes the Kubernetes
// projected-volume bookkeeping ("..data", "..2025_01_01_..." temp dirs) as well
// as dotfiles, while the real config keys (symlinks into "..data") are read.
func (c *configFileController) readDir(cs configSource, m map[string]string) error {
	entries, err := os.ReadDir(cs.path)
	if err != nil {
		return errutil.Explain(err, "file-watch: read dir %s failed", cs.path)
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(cs.path, name)
		fi, statErr := os.Stat(full)
		if statErr != nil || fi.IsDir() {
			continue
		}
		if cs.format == "" {
			if _, ok := contentReaders[strings.TrimPrefix(filepath.Ext(name), ".")]; !ok {
				continue
			}
		}
		if err = c.readOneFile(full, cs.format, m); err != nil {
			return err
		}
	}
	return nil
}

// readOneFile parses a single file (by forced format, or by extension when
// format is empty) and merges its flattened keys into m.
func (c *configFileController) readOneFile(path, format string, m map[string]string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return errutil.Explain(err, "file-watch: read %s failed", path)
	}
	r := contentReaders[format] // nil when format == ""
	if r == nil {
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		var ok bool
		if r, ok = contentReaders[ext]; !ok {
			return errutil.Explain(nil, "file-watch: unsupported file type %q", path)
		}
	}
	parsed, err := r(b)
	if err != nil {
		return errutil.Explain(err, "file-watch: parse %s failed", path)
	}
	maps.Copy(m, flatten.Flatten(parsed))
	return nil
}
