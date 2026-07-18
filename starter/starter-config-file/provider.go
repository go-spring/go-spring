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
	"sync"
	"sync/atomic"

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
	// and, whenever the mount changes, triggers a full property refresh. This is
	// the piece that makes a Kubernetes ConfigMap/Secret mount hot-reloadable:
	// the kubelet updates the volume by atomically swapping the "..data" symlink,
	// and the directory watcher (see watch.go) turns that into a refresh.
	conf.RegisterProvider("file-watch", loadWatchedConfig)
}

// contentReader parses raw configuration bytes into a nested map based on a
// declared format name (rather than a file extension). It is only used when the
// caller forces a format via the "format" query parameter; otherwise files are
// parsed by extension through conf/reader.
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

// refreshHook holds the callback used to reload application properties when a
// watched mount changes. It is populated by the refresh bridge bean during
// container wiring (see starter.go). A change that arrives before the bridge is
// wired is safely ignored; the value is picked up on the next refresh.
var refreshHook atomic.Pointer[func() error]

// setRefreshHook installs the callback that reloads application properties.
func setRefreshHook(fn func() error) {
	refreshHook.Store(&fn)
}

// triggerRefresh invokes the installed refresh callback, if any.
func triggerRefresh() {
	if p := refreshHook.Load(); p != nil {
		_ = (*p)()
	}
}

// configSource holds the parsed components of a file-watch provider source.
type configSource struct {
	path   string // absolute or relative path to a directory or a single file
	format string // optional format override applied to all files
}

// parseSource parses a provider source of the form
//
//	<path>[?format=..]
//
// The leading "file-watch:" prefix has already been stripped by
// conf/provider.Load. A Windows-style path is not supported; K8s mounts are
// POSIX paths.
func parseSource(source string) (configSource, error) {
	path := source
	var format string
	// Only treat a trailing "?..." as a query string; real paths do not contain
	// '?'. This keeps parsing dependency-free and predictable for mount paths.
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

// watched tracks directories that already have a change watcher, so repeated
// Load calls (startup + every RefreshProperties) do not start duplicate
// watchers. Guarded by watchedMu.
var (
	watchedMu sync.Mutex
	watched   = map[string]struct{}{}
)

// loadWatchedConfig implements conf/provider.Provider. It reads configuration
// from the mounted path, parses it, and installs a directory watcher that
// triggers an application property refresh on change.
func loadWatchedConfig(optional bool, source string) (map[string]string, error) {
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
	ensureWatch(watchDir)

	m := map[string]string{}
	if info.IsDir() {
		if err = readDir(cs, m); err != nil {
			return nil, err
		}
	} else {
		if err = readOneFile(cs.path, cs.format, m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// readDir merges every eligible file in a mounted directory into m. Entries
// whose name begins with '.' are skipped: this excludes the Kubernetes
// projected-volume bookkeeping ("..data", "..2025_01_01_..." temp dirs) as well
// as dotfiles, while the real config keys (symlinks into "..data") are read.
func readDir(cs configSource, m map[string]string) error {
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
		// Follow the symlink/entry; skip anything that resolves to a directory.
		fi, statErr := os.Stat(full)
		if statErr != nil || fi.IsDir() {
			continue
		}
		// Without a forced format, silently skip files with no known extension
		// so unrelated keys in the same mount do not fail the load.
		if cs.format == "" {
			if _, ok := contentReaders[strings.TrimPrefix(filepath.Ext(name), ".")]; !ok {
				continue
			}
		}
		if err = readOneFile(full, cs.format, m); err != nil {
			return err
		}
	}
	return nil
}

// readOneFile parses a single file (by forced format, or by extension when
// format is empty) and merges its flattened keys into m.
func readOneFile(path, format string, m map[string]string) error {
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
