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
	"net/url"
	"os"
	"path/filepath"
	"testing"

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
