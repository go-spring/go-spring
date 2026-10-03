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
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ParsesByExtension(t *testing.T) {
	ctl := newFileWatchCtrl()

	root := t.TempDir()
	path := filepath.Join(root, "app.properties")
	if err := os.WriteFile(path, []byte("a=1\nb=2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m, err := ctl.Load(false, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m["a"] != "1" || m["b"] != "2" {
		t.Errorf("got %#v, want a=1 b=2", m)
	}
}

func TestLoad_YamlNestedFlattened(t *testing.T) {
	ctl := newFileWatchCtrl()
	root := t.TempDir()
	path := filepath.Join(root, "app.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m, err := ctl.Load(false, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m["server.port"] != "8080" {
		t.Errorf("got %#v, want server.port=8080", m)
	}
}

func TestLoad_UnsupportedExtensionErrors(t *testing.T) {
	ctl := newFileWatchCtrl()

	root := t.TempDir()
	path := filepath.Join(root, "readme.md") // no reader registered for .md
	if err := os.WriteFile(path, []byte("# hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ctl.Load(false, path); err == nil {
		t.Fatalf("Load on .md must error")
	}
}

func TestLoad_EmptyPathErrors(t *testing.T) {
	ctl := newFileWatchCtrl()

	if _, err := ctl.Load(false, ""); err == nil {
		t.Fatalf("Load on empty path must error")
	}
}

func TestLoad_OptionalMissingReturnsNil(t *testing.T) {
	ctl := newFileWatchCtrl()

	m, err := ctl.Load(true, filepath.Join(t.TempDir(), "nope"))
	if err != nil || m != nil {
		t.Fatalf("optional missing: m=%v err=%v", m, err)
	}
}

// watchCount reports how many watchers the core currently holds.
func watchCount(c *watchCore) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.watchers)
}

// TestCloseReleasesWatchersAndLoadRearms covers the provider lifecycle: Close
// must release every watcher (a shut-down app leaves nothing behind), and the
// next Load — a new application instance in the same process — must watch again.
func TestCloseReleasesWatchersAndLoadRearms(t *testing.T) {
	ctl := newFileWatchCtrl()
	root := t.TempDir()
	path := filepath.Join(root, "app.properties")
	if err := os.WriteFile(path, []byte("a=1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := ctl.Load(false, path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if n := watchCount(&ctl.watchCore); n != 1 {
		t.Fatalf("after Load: %d watchers, want 1", n)
	}

	if err := ctl.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if n := watchCount(&ctl.watchCore); n != 0 {
		t.Errorf("after Close: %d watchers, want 0", n)
	}

	if _, err := ctl.Load(false, path); err != nil {
		t.Fatalf("Load after Close: %v", err)
	}
	if n := watchCount(&ctl.watchCore); n != 1 {
		t.Errorf("after Load following Close: %d watchers, want 1", n)
	}
	if err := ctl.Close(); err != nil {
		t.Fatalf("final Close: %v", err)
	}
}
