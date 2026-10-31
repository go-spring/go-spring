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
	"sync/atomic"
	"testing"
	"time"
)

const testDocV1 = `spring.test.client.timeout=100ms
`

const testDocV2 = `spring.test.client.timeout=300ms
spring.test.client.rules[0].name=slow
spring.test.client.rules[0].rate=0.5
`

const testDocV2YAML = `spring:
  test:
    client:
      timeout: 300ms
      rules:
        - name: slow
          rate: 0.5
`

func writeDoc(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// deliveries is the contract-legal observation channel for these tests: a
// subscription records each delivered document; reading last after a receive
// from ch is ordered by the channel. Polling Snapshot() from here would race
// the delivery goroutine — exactly what the serialized-deliveries contract
// forbids.
type deliveries struct {
	ch   chan testDoc
	n    atomic.Int32
	last testDoc
}

func watchDeliveries(s *FileSource[testDoc]) *deliveries {
	d := &deliveries{ch: make(chan testDoc, 16)}
	s.Subscribe(func(_ context.Context, doc testDoc) {
		d.n.Add(1)
		d.last = doc
		select {
		case d.ch <- doc:
		default:
		}
	})
	return d
}

// awaitTimeout waits until a delivered document's timeout reaches d (fsnotify
// delivery is asynchronous) or the deadline passes.
func (d *deliveries) awaitTimeout(t *testing.T, want time.Duration) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-d.ch:
		case <-time.After(20 * time.Millisecond):
		}
		if d.last.Client.Timeout == want {
			return
		}
	}
	t.Fatalf("delivered timeout: want %v, got %v", want, d.last.Client.Timeout)
}

// TestFileSource_InitialSnapshot pins the constructor's first read: by the time
// NewFileSource returns, the snapshot is the document the file actually holds.
func TestFileSource_InitialSnapshot(t *testing.T) {
	s, err := NewFileSource[testDoc](writeDoc(t, "doc.properties", testDocV1))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if got := s.Snapshot().Client.Timeout; got != 100*time.Millisecond {
		t.Fatalf("initial snapshot wrong: %v", got)
	}
}

// TestFileSource_HotReload covers the refresh chain end to end: an atomic-rename
// edit (the harder case, versus an in-place write) reloads, re-parses, swaps the
// snapshot and pushes to the subscriber.
func TestFileSource_HotReload(t *testing.T) {
	path := writeDoc(t, "doc.properties", testDocV1)
	s, err := NewFileSource[testDoc](path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	d := watchDeliveries(s)

	// Atomic-rename edit, the harder case vs in-place write.
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, []byte(testDocV2), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}

	d.awaitTimeout(t, 300*time.Millisecond)
	if len(d.last.Client.Rules) != 1 || d.last.Client.Rules[0].Name != "slow" {
		t.Fatalf("pushed document should carry the new rules: %+v", d.last)
	}
}

// TestFileSource_BadEditKeepsLastGood pins the contract "everything pushed is
// vouched for": an UNPARSEABLE edit keeps the last good snapshot and pushes
// nothing. An empty file is not a parse failure — the whole document is bound,
// so it is a legitimate "off" document and pushes the zero value; a subsequent
// good edit recovers.
func TestFileSource_BadEditKeepsLastGood(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.yaml")
	if err := os.WriteFile(path, []byte("spring:\n  test:\n    client:\n      timeout: 100ms\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewFileSource[testDoc](path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	d := watchDeliveries(s)

	// Malformed YAML: a hard parse failure — nothing is delivered.
	if err = os.WriteFile(path, []byte("spring.test: { unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	if d.n.Load() != 0 {
		t.Fatalf("malformed edit must push nothing, got %d", d.n.Load())
	}

	// Empty file: valid, binds zero, and pushes — the disarm spelling.
	if err = os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	d.awaitTimeout(t, 0)

	// Recovery: a good edit after the disarm pushes again.
	if err = os.WriteFile(path, []byte(testDocV2YAML), 0o644); err != nil {
		t.Fatal(err)
	}
	d.awaitTimeout(t, 300*time.Millisecond)
}

// TestFileSource_TouchDoesNotPush pins the change-dedup: rewriting the same
// bytes (a touch) must not push again. The document carries a slice, so this
// also pins that the comparison is by value (reflect.DeepEqual), not identity.
func TestFileSource_TouchDoesNotPush(t *testing.T) {
	path := writeDoc(t, "doc.properties", testDocV2)
	s, err := NewFileSource[testDoc](path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	var pushes atomic.Int32
	s.Subscribe(func(context.Context, testDoc) { pushes.Add(1) })

	// Same content, new mtime: a "touch" must be a no-op.
	if err = os.WriteFile(path, []byte(testDocV2), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	if pushes.Load() != 0 {
		t.Fatalf("identical reload must not push, got %d", pushes.Load())
	}
}

// TestFileSource_MissingFileFailsFast pins the startup behavior: a broken path
// surfaces at construction, not as a silently disabled root.
func TestFileSource_MissingFileFailsFast(t *testing.T) {
	_, err := NewFileSource[testDoc](filepath.Join(t.TempDir(), "nope.properties"))
	if err == nil {
		t.Fatal("missing document should fail construction")
	}
}
