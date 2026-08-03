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
	root := t.TempDir()
	path := filepath.Join(root, "app.properties")
	if err := os.WriteFile(path, []byte("a=1\nb=2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m, err := fileWatchController.Load(false, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m["a"] != "1" || m["b"] != "2" {
		t.Errorf("got %#v, want a=1 b=2", m)
	}
}

func TestLoad_YamlNestedFlattened(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	m, err := fileWatchController.Load(false, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m["server.port"] != "8080" {
		t.Errorf("got %#v, want server.port=8080", m)
	}
}

func TestLoad_UnsupportedExtensionErrors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "readme.md") // no reader registered for .md
	if err := os.WriteFile(path, []byte("# hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := fileWatchController.Load(false, path); err == nil {
		t.Fatalf("Load on .md must error")
	}
}

func TestLoad_EmptyPathErrors(t *testing.T) {
	if _, err := fileWatchController.Load(false, ""); err == nil {
		t.Fatalf("Load on empty path must error")
	}
}

func TestLoad_OptionalMissingReturnsNil(t *testing.T) {
	m, err := fileWatchController.Load(true, filepath.Join(t.TempDir(), "nope"))
	if err != nil || m != nil {
		t.Fatalf("optional missing: m=%v err=%v", m, err)
	}
}
