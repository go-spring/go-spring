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
	"runtime"
	"slices"
	"testing"
)

// writeLeaf writes content into relpath under root, creating parent dirs.
func writeLeaf(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func TestWalkConfigTree_NestedKeysAndTrim(t *testing.T) {
	root := t.TempDir()
	writeLeaf(t, root, "db/user", "alice\n")
	writeLeaf(t, root, "db/password", "s3cr3t\n\n")
	writeLeaf(t, root, "server/port", "8080")
	writeLeaf(t, root, "flat", "  spaced  \n")

	// dot-prefixed file and dir must be skipped (simulates ..data bookkeeping)
	writeLeaf(t, root, ".hidden", "ignore")
	writeLeaf(t, root, ".data/leaf", "ignore")

	var watched []string
	m, err := walkConfigTree(root, func(dir string) { watched = append(watched, dir) })
	if err != nil {
		t.Fatalf("walkConfigTree: %v", err)
	}

	want := map[string]string{
		"db.user":       "alice",
		"db.password":   "s3cr3t",
		"server.port":   "8080",
		"flat":          "spaced", // TrimSpace, not just trailing newline
	}
	for k, v := range want {
		if got := m[k]; got != v {
			t.Errorf("key %q = %q, want %q", k, got, v)
		}
	}
	if _, ok := m["hidden"]; ok {
		t.Errorf("dotfile .hidden should be skipped, got key %q", "hidden")
	}
	if len(m) != len(want) {
		t.Errorf("got %d keys, want %d (extra: %#v)", len(m), len(want), m)
	}

	// root, db, server are directories that should be watched.
	wantWatched := []string{root, filepath.Join(root, "db"), filepath.Join(root, "server")}
	if runtime.GOOS != "windows" { // symlink/skip behavior verified on unix
		for _, w := range wantWatched {
			if !slices.Contains(watched, w) {
				t.Errorf("expected %s to be watched, got %v", w, watched)
			}
		}
	}
}

func TestProviderTypeSymmetry(t *testing.T) {
	tmp := t.TempDir()

	// file-watch rejects a directory.
	if _, err := fileWatchController.Load(false, tmp); err == nil {
		t.Fatalf("file-watch on a directory must error")
	}

	// configtree rejects a file.
	file := filepath.Join(tmp, "scalar")
	if err := os.WriteFile(file, []byte("v"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := fileWatchController.LoadConfigTree(false, file); err == nil {
		t.Fatalf("configtree on a file must error")
	}

	// optional:true tolerates a missing path for both providers.
	if m, err := fileWatchController.LoadConfigTree(true, filepath.Join(tmp, "nope")); err != nil || m != nil {
		t.Fatalf("optional missing configtree: m=%v err=%v", m, err)
	}
}

func TestWalkConfigTree_SymlinkLeafFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is unix-only")
	}
	root := t.TempDir()
	// Real data dir (like a K8s timestamped data dir), plus a key symlink that
	// points through it the way a ConfigMap/Secret mount does.
	writeLeaf(t, root, "..2026/data", "real-value\n")
	keyLink := filepath.Join(root, "data")
	if err := os.Symlink(filepath.Join("..2026", "data"), keyLink); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	m, err := walkConfigTree(root, nil)
	if err != nil {
		t.Fatalf("walkConfigTree: %v", err)
	}
	// "..2026" is skipped (dot prefix); the "data" symlink leaf is read through.
	if got := m["data"]; got != "real-value" {
		t.Errorf("m[data] = %q, want %q", got, "real-value")
	}
	if _, ok := m["2026.data"]; ok {
		t.Errorf("dot-prefixed dir ..2026 should be skipped")
	}
	if len(m) != 1 {
		t.Errorf("got %d keys, want 1: %#v", len(m), m)
	}
}
