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
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
)

const rulesV1 = `govern.enabled=true
govern.client.default.attempt-timeout=100ms
`

const rulesV2 = `govern.enabled=true
govern.client.default.attempt-timeout=300ms
govern.client.default.max-retries=1
`

// YAML variants of the same rules, to exercise format-by-extension and the
// malformed/empty cases (properties syntax almost never hard-fails).
const rulesV1YAML = `govern:
  enabled: true
  client:
    default:
      attempt-timeout: 100ms
`

const rulesV2YAML = `govern:
  enabled: true
  client:
    default:
      attempt-timeout: 300ms
      max-retries: 1
`

// writeRules writes a rules file and returns its path.
func writeRules(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "govern.properties")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// awaitCfg polls until the source snapshot's default timeout reaches d (fsnotify
// delivery is asynchronous) or the deadline passes.
func awaitCfg(t *testing.T, s *FileSource, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cfg := s.Snapshot(); cfg.Client.Default.AttemptTimeout == d {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	cfg := s.Snapshot()
	t.Fatalf("snapshot default timeout: want %v, got %v", d, cfg.Client.Default.AttemptTimeout)
}

func TestFileSource_InitialSnapshot(t *testing.T) {
	s, err := NewFileSource(writeRules(t, rulesV1))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	cfg := s.Snapshot()
	if !cfg.Enabled || cfg.Client.Default.AttemptTimeout != 100*time.Millisecond {
		t.Fatalf("initial snapshot wrong: %+v", cfg)
	}
}

// TestFileSource_HotReload covers the core of the self-built chain: a file edit
// (via rename, like an editor save) reloads, re-parses, swaps the snapshot and
// pushes to the subscriber.
func TestFileSource_HotReload(t *testing.T) {
	path := writeRules(t, rulesV1)
	s, err := NewFileSource(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.Start(); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var got governance.Config
	s.Subscribe(func(cfg governance.Config) { mu.Lock(); got = cfg; mu.Unlock() })

	// Atomic-rename edit, the harder case vs in-place write.
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, []byte(rulesV2), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}

	awaitCfg(t, s, 300*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if got.Client.Default.MaxRetries != 1 {
		t.Fatalf("pushed config should carry the new rules: %+v", got.Client.Default)
	}
}

// TestFileSource_BadEditKeepsLastGood pins the contract "everything pushed is
// vouched for": an unparseable edit and an empty (truncated) file both keep
// the last good snapshot and push nothing; a subsequent good edit recovers.
func TestFileSource_BadEditKeepsLastGood(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govern.yaml")
	if err := os.WriteFile(path, []byte(rulesV1YAML), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewFileSource(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.Start(); err != nil {
		t.Fatal(err)
	}

	var pushes int
	s.Subscribe(func(governance.Config) { pushes++ })

	// awaitStable waits until the file's mtime is old enough that the watcher
	// has certainly processed it, plus a beat.
	awaitStable := func() {
		time.Sleep(400 * time.Millisecond)
	}

	// Malformed YAML: a hard parse failure.
	if err = os.WriteFile(path, []byte("govern: { unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}
	awaitStable()
	if cfg := s.Snapshot(); cfg.Client.Default.AttemptTimeout != 100*time.Millisecond {
		t.Fatalf("malformed edit must keep last good snapshot: %+v", cfg.Client.Default)
	}

	// Empty file: parses "fine" but carries no govern.* keys — a truncating
	// write, not a legitimate disable (that would be govern.enabled=false).
	if err = os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	awaitStable()
	if cfg := s.Snapshot(); cfg.Client.Default.AttemptTimeout != 100*time.Millisecond {
		t.Fatalf("empty edit must keep last good snapshot: %+v", cfg.Client.Default)
	}
	if pushes != 0 {
		t.Fatalf("no bad edit may push, got %d pushes", pushes)
	}

	// Recovery: a good edit after bad ones works.
	if err = os.WriteFile(path, []byte(rulesV2YAML), 0o644); err != nil {
		t.Fatal(err)
	}
	awaitCfg(t, s, 300*time.Millisecond)
	if pushes != 1 {
		t.Fatalf("recovered edit should push exactly once, got %d", pushes)
	}
}

// TestFileSource_MissingFileFailsFast pins the startup behavior: a broken path
// surfaces at construction, not as a silently disabled center.
func TestFileSource_MissingFileFailsFast(t *testing.T) {
	_, err := NewFileSource(filepath.Join(t.TempDir(), "nope.properties"))
	if err == nil {
		t.Fatal("missing rules file should fail construction")
	}
}

// TestFileSource_EndToEndWithCenter wires the source into a governance center
// built directly over its own module authorities (the same Center.SetSource
// contract the starter's bean injection drives) and asserts the fan-out reaches
// a subscribed label on a live file edit. There is no process-global to reset:
// the center and its authorities belong to this test alone.
func TestFileSource_EndToEndWithCenter(t *testing.T) {
	path := writeRules(t, rulesV1)
	s, err := NewFileSource(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.Start(); err != nil {
		t.Fatal(err)
	}

	res := resilience.NewManager()
	ctr := governance.NewCenter(governance.Config{},
		res, loadbalance.NewManager(), fault.NewInjector(fault.Configs{Client: fault.Config{}}, nil))
	ctr.SetSource(s) // adopts the source's initial snapshot, arming res

	var mu sync.Mutex
	var got resilience.ClientPolicy
	res.Subscribe("demo:service", func(p resilience.ClientPolicy) { mu.Lock(); got = p; mu.Unlock() })
	mu.Lock()
	if got.AttemptTimeout != 100*time.Millisecond {
		mu.Unlock()
		t.Fatalf("armed policy: want 100ms, got %v", got.AttemptTimeout)
	}
	mu.Unlock()

	if err = os.WriteFile(path, []byte(rulesV2), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := got.AttemptTimeout == 300*time.Millisecond
		mu.Unlock()
		if done {
			return // fan-out delivered
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("hot-reload did not fan out to the registered label")
}
