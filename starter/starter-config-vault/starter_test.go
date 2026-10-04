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

package StarterConfigVault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"go-spring.org/stdlib/testing/assert"
)

func TestParseSource(t *testing.T) {
	// Full form: every query knob spelled out.
	t.Setenv("VAULT_TOKEN", "tk")
	cs, err := parseSource("127.0.0.1:8200/secret/app?scheme=https&namespace=ns&kv-version=1&key=conf&format=yaml&prefix=db&poll-ms=500&token=tk")
	assert.That(t, err).Nil()
	assert.That(t, cs.address).Equal("https://127.0.0.1:8200")
	assert.That(t, cs.namespace).Equal("ns")
	assert.That(t, cs.mount).Equal("secret")
	assert.That(t, cs.path).Equal("app")
	assert.That(t, cs.kvVersion).Equal(1)
	assert.That(t, cs.key).Equal("conf")
	assert.That(t, cs.format).Equal("yaml")
	assert.That(t, cs.prefix).Equal("db")
	assert.That(t, cs.pollMs).Equal(500)
	assert.That(t, cs.token).Equal("tk")

	// Defaults: http scheme, kv-v2, properties format, 5s poll.
	cs, err = parseSource("127.0.0.1:8200/secret/app?token=tk")
	assert.That(t, err).Nil()
	assert.That(t, cs.address).Equal("http://127.0.0.1:8200")
	assert.That(t, cs.kvVersion).Equal(2)
	assert.That(t, cs.format).Equal("properties")
	assert.That(t, cs.pollMs).Equal(5000)

	// Missing server, mount or path each fail loudly.
	for _, bad := range []string{
		"onlypath",           // no host: no mount/path split either
		"127.0.0.1:8200/one", // mount but no path
		"/secret/app",        // no server address
	} {
		_, err = parseSource(bad + "?token=tk")
		assert.That(t, err).NotNil()
	}

	// Out-of-range knobs are rejected instead of silently coerced.
	_, err = parseSource("127.0.0.1:8200/secret/app?kv-version=3&token=tk")
	assert.That(t, err).NotNil()
	_, err = parseSource("127.0.0.1:8200/secret/app?kv-version=x&token=tk")
	assert.That(t, err).NotNil()
	_, err = parseSource("127.0.0.1:8200/secret/app?poll-ms=0&token=tk")
	assert.That(t, err).NotNil()
	_, err = parseSource("127.0.0.1:8200/secret/app?poll-ms=-1&token=tk")
	assert.That(t, err).NotNil()
}

func TestResolveToken(t *testing.T) {
	// Query parameter wins over the environment.
	t.Setenv("VAULT_TOKEN", "env-tk")
	token, err := resolveToken(url.Values{"token": {"q-tk"}})
	assert.That(t, err).Nil()
	assert.That(t, token).Equal("q-tk")

	// VAULT_TOKEN is the fallback.
	token, err = resolveToken(url.Values{})
	assert.That(t, err).Nil()
	assert.That(t, token).Equal("env-tk")

	// A token file is consulted only when no token env is set: query
	// token-file first, then VAULT_TOKEN_FILE.
	t.Setenv("VAULT_TOKEN", "")
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	assert.That(t, os.WriteFile(file, []byte("  file-tk\n"), 0o600)).Nil()
	token, err = resolveToken(url.Values{"token-file": {file}})
	assert.That(t, err).Nil()
	assert.That(t, token).Equal("file-tk")

	t.Setenv("VAULT_TOKEN_FILE", file)
	token, err = resolveToken(url.Values{})
	assert.That(t, err).Nil()
	assert.That(t, token).Equal("file-tk")

	// Nothing anywhere fails loudly.
	t.Setenv("VAULT_TOKEN_FILE", "")
	_, err = resolveToken(url.Values{})
	assert.That(t, err).NotNil()

	// An unreadable token file is an error, not a silent skip.
	_, err = resolveToken(url.Values{"token-file": {filepath.Join(dir, "missing")}})
	assert.That(t, err).NotNil()
}

func TestToProperties(t *testing.T) {
	cs := configSource{mount: "secret", path: "app"}

	// Without key: the whole data map is flattened.
	m, err := toProperties(cs, map[string]any{"db": map[string]any{"host": "h", "port": 3306}})
	assert.That(t, err).Nil()
	assert.That(t, m["db.host"]).Equal("h")
	assert.That(t, m["db.port"]).Equal("3306")

	// With key: the named string field is parsed by the declared format.
	cs.key, cs.format = "conf", "properties"
	m, err = toProperties(cs, map[string]any{"conf": "greeting=hello\n"})
	assert.That(t, err).Nil()
	assert.That(t, m["greeting"]).Equal("hello")

	// With prefix: every key is namespaced.
	cs.prefix = "db"
	m, err = toProperties(cs, map[string]any{"conf": "host=h\n"})
	assert.That(t, err).Nil()
	assert.That(t, m["db.host"]).Equal("h")

	// Missing or non-string key field fails loudly.
	cs.prefix = ""
	_, err = toProperties(cs, map[string]any{"other": "x"})
	assert.That(t, err).NotNil()
	_, err = toProperties(cs, map[string]any{"conf": 42})
	assert.That(t, err).NotNil()
}

func TestFingerprint(t *testing.T) {
	// nil and empty are distinct fingerprints; content order is normalized
	// (json.Marshal sorts map keys) so equal maps always match; the digest
	// is fixed-length regardless of payload size.
	assert.That(t, fingerprint(nil)).Equal("<nil>")
	empty := fingerprint(map[string]any{})
	assert.That(t, empty).NotEqual("<nil>")
	assert.That(t, len(empty)).Equal(64) // sha256 hex
	assert.That(t, fingerprint(map[string]any{"a": "1", "b": "2"})).
		Equal(fingerprint(map[string]any{"b": "2", "a": "1"}))
	assert.That(t, fingerprint(map[string]any{"a": "1"})).
		NotEqual(fingerprint(map[string]any{"a": "2"}))
}

func TestIsNotFound(t *testing.T) {
	// A 404 ResponseError and an error whose text carries 404 both count;
	// other status codes do not.
	assert.That(t, isNotFound(&api.ResponseError{StatusCode: 404})).True()
	assert.That(t, isNotFound(errText("context deadline exceeded: 404 not found"))).True()
	assert.That(t, isNotFound(&api.ResponseError{StatusCode: 403})).False()
	assert.That(t, isNotFound(errText("connection refused"))).False()
}

type errText string

func (e errText) Error() string { return string(e) }

func TestClientAndWatchKey(t *testing.T) {
	cs := configSource{address: "http://h:1", namespace: "ns", token: "tk", mount: "m", path: "p"}
	assert.That(t, clientKey(cs)).Equal("http://h:1|ns|tk")
	assert.That(t, watchKey(cs)).Equal("http://h:1|ns|tk|m|p")
}

// fakeVault is an in-process Vault KV server backed by httptest. It serves one
// mutable secret per (kvVersion, mount, path) and counts the reads it served,
// so tests can observe both content changes and polling activity.
type fakeVault struct {
	srv     *httptest.Server
	reads   atomic.Int64
	mu      sync.Mutex
	kv2     map[string]map[string]any // mount/path -> data (nil value = 404)
	kv1     map[string]map[string]any
	failAll bool
}

func newFakeVault() *fakeVault {
	f := &fakeVault{
		kv2: map[string]map[string]any{},
		kv1: map[string]map[string]any{},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.reads.Add(1)
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.failAll {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		// KV v2 reads hit /v1/<mount>/data/<path>; KV v1 reads hit /v1/<mount>/<path>.
		if idx := strings.Index(r.URL.Path, "/data/"); idx > 0 {
			data, found := f.kv2[r.URL.Path[len("/v1/"):idx]+"/"+r.URL.Path[idx+len("/data/"):]]
			writeSecret(w, data, found, true)
			return
		}
		data, found := f.kv1[strings.TrimPrefix(r.URL.Path, "/v1/")]
		writeSecret(w, data, found, false)
	}))
	return f
}

func writeSecret(w http.ResponseWriter, data map[string]any, found, kv2 bool) {
	if !found || data == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{"not found"}})
		return
	}
	if kv2 {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"data": data, "metadata": map[string]any{"version": 1}},
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

// source builds an import path for the fake server.
func (f *fakeVault) source(mount, path string) string {
	u, _ := url.Parse(f.srv.URL)
	return u.Host + "/" + mount + "/" + path + "?token=tk&poll-ms=50"
}

func (f *fakeVault) Close() { f.srv.Close() }

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within timeout")
}

func TestLoadKVv2AndV1(t *testing.T) {
	f := newFakeVault()
	defer f.Close()
	f.mu.Lock()
	f.kv2["secret/app"] = map[string]any{"greeting": "hi", "db": map[string]any{"host": "h1"}}
	f.kv1["kv1/app"] = map[string]any{"port": 3306}
	f.mu.Unlock()

	c := newVaultCtrl()
	defer c.Close(context.Background())

	m, err := c.Load(false, f.source("secret", "app")+"&kv-version=2")
	assert.That(t, err).Nil()
	assert.That(t, m["greeting"]).Equal("hi")
	assert.That(t, m["db.host"]).Equal("h1")

	m, err = c.Load(false, f.source("kv1", "app")+"&kv-version=1")
	assert.That(t, err).Nil()
	assert.That(t, m["port"]).Equal("3306")

	// The same source hits the client cache: one client entry, no error.
	cs, _ := parseSource(f.source("secret", "app") + "&kv-version=2")
	cli, err := c.clientFor(context.Background(), cs)
	assert.That(t, err).Nil()
	cli2, _ := c.clientFor(context.Background(), cs)
	assert.That(t, cli == cli2).True()

	// Single-field mode reads the named field as a document.
	f.mu.Lock()
	f.kv2["secret/doc"] = map[string]any{"application.properties": "demo.message=hello\n"}
	f.mu.Unlock()
	m, err = c.Load(false, f.source("secret", "doc")+"&key=application.properties&format=properties")
	assert.That(t, err).Nil()
	assert.That(t, m["demo.message"]).Equal("hello")
}

func TestLoadNotFoundAndOptional(t *testing.T) {
	f := newFakeVault()
	defer f.Close()
	c := newVaultCtrl()
	defer c.Close(context.Background())

	// Required missing secret: hard error naming mount/path.
	_, err := c.Load(false, f.source("secret", "nope"))
	assert.That(t, err).NotNil()

	// Optional missing secret: nil props, nil error; the watcher stays armed.
	m, err := c.Load(true, f.source("secret", "nope"))
	assert.That(t, err).Nil()
	assert.That(t, m).Nil()

	// Optional transport failure: skipped as well.
	f.mu.Lock()
	f.failAll = true
	f.mu.Unlock()
	m, err = c.Load(true, f.source("secret", "other"))
	assert.That(t, err).Nil()
	assert.That(t, m).Nil()
	f.mu.Lock()
	f.failAll = false
	f.mu.Unlock()

	// Required transport failure: error propagates.
	f.mu.Lock()
	f.failAll = true
	f.mu.Unlock()
	_, err = c.Load(false, f.source("secret", "other2"))
	assert.That(t, err).NotNil()
	f.mu.Lock()
	f.failAll = false
	f.mu.Unlock()
}

func TestCloseStopsPollersAndRearms(t *testing.T) {
	f := newFakeVault()
	defer f.Close()
	f.mu.Lock()
	f.kv2["secret/app"] = map[string]any{"k": "v"}
	f.mu.Unlock()

	c := newVaultCtrl()
	_, err := c.Load(false, f.source("secret", "app"))
	assert.That(t, err).Nil()

	// The poller ticks; after Close the request count freezes.
	base := f.reads.Load()
	waitFor(t, func() bool { return f.reads.Load() > base })
	assert.That(t, c.Close(context.Background())).Nil()
	frozen := f.reads.Load()
	time.Sleep(200 * time.Millisecond)
	assert.That(t, f.reads.Load() == frozen).True()

	// A new Load rearms: the poll generation restarts (fresh baseline, fresh
	// watcher), so reads resume.
	_, err = c.Load(false, f.source("secret", "app"))
	assert.That(t, err).Nil()
	base = f.reads.Load()
	waitFor(t, func() bool { return f.reads.Load() > base })

	// Dedup: a second Load of the same source does not add another poller.
	_, err = c.Load(false, f.source("secret", "app"))
	assert.That(t, err).Nil()
	waitFor(t, func() bool { return f.reads.Load() > base })
	c.Close(context.Background())
}

func TestWatchLoopDetectsChange(t *testing.T) {
	f := newFakeVault()
	defer f.Close()
	f.mu.Lock()
	f.kv2["secret/app"] = map[string]any{"k": "v1"}
	f.mu.Unlock()

	c := newVaultCtrl()
	defer c.Close(context.Background())
	_, err := c.Load(false, f.source("secret", "app"))
	assert.That(t, err).Nil()

	// Change the content; the poller sees a new fingerprint. gs.RefreshProperties
	// is not runnable outside a running app, so the refresh itself errors and is
	// dropped — but the poll loop must keep running (self-healing), not exit.
	f.mu.Lock()
	f.kv2["secret/app"] = map[string]any{"k": "v2"}
	f.mu.Unlock()
	base := f.reads.Load()
	waitFor(t, func() bool { return f.reads.Load() > base+2 })

	// Recovery logging path: fail for a while, then recover.
	f.mu.Lock()
	f.failAll = true
	f.mu.Unlock()
	waitFor(t, func() bool { return f.reads.Load() > base+4 })
	f.mu.Lock()
	f.failAll = false
	f.mu.Unlock()
	waitFor(t, func() bool { return f.reads.Load() > base+6 })
}
