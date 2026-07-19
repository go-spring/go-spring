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
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/vault/api"
	"go-spring.org/spring/conf"
	confjson "go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register "vault" as a remote configuration provider so that a
	// spring.app.imports entry such as
	//
	//	optional:vault:127.0.0.1:8200/secret/gs-config-demo?kv-version=2
	//
	// pulls configuration from a HashiCorp Vault KV secret at startup and on
	// every RefreshProperties call. The Vault token is taken from the
	// environment (VAULT_TOKEN / a token file) by default so it never lives in
	// a configuration file.
	conf.RegisterProvider("vault", loadVaultConfig)
}

// contentReader parses raw configuration bytes into a nested map based on the
// declared format. It is used only in single-field mode (?key=...), where one
// secret field holds a document; whole-secret mode maps fields directly to
// properties.
type contentReader func(b []byte) (map[string]any, error)

var contentReaders = map[string]contentReader{
	"properties": prop.Read,
	"props":      prop.Read,
	"yaml":       yaml.Read,
	"yml":        yaml.Read,
	"toml":       toml.Read,
	"tml":        toml.Read,
	"json":       confjson.Read,
}

// refreshHook holds the callback used to reload application properties when a
// watched secret changes. It is populated by the refresh bridge bean during
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

// configSource holds the parsed components of a vault provider source string.
type configSource struct {
	address   string // scheme://host:port of the Vault server
	namespace string // enterprise namespace, empty for OSS
	token     string // resolved Vault token
	mount     string // KV mount point, e.g. "secret"
	path      string // secret path under the mount
	kvVersion int    // 1 or 2
	key       string // optional single field holding a document
	format    string // format of that field (single-field mode)
	prefix    string // optional prefix prepended to produced property keys
	pollMs    int    // polling interval for change detection
}

// parseSource parses a provider source of the form
//
//	<host>:<port>/<mount>/<path>?kv-version=..&token=..&namespace=..&scheme=..&key=..&format=..&prefix=..&poll-ms=..
//
// The leading "vault:" prefix has already been stripped by conf/provider.Load.
func parseSource(source string) (configSource, error) {
	u, err := url.Parse("vault://" + source)
	if err != nil {
		return configSource{}, errutil.Explain(err, "invalid vault source %q", source)
	}
	if u.Host == "" {
		return configSource{}, errutil.Explain(nil, "missing vault server address in %q", source)
	}
	full := strings.TrimPrefix(u.Path, "/")
	mount, path, ok := strings.Cut(full, "/")
	if !ok || mount == "" || path == "" {
		return configSource{}, errutil.Explain(nil, "vault path must be <mount>/<path>, got %q", full)
	}

	q := u.Query()
	scheme := q.Get("scheme")
	if scheme == "" {
		scheme = "http"
	}
	cs := configSource{
		address:   scheme + "://" + u.Host,
		namespace: q.Get("namespace"),
		mount:     mount,
		path:      path,
		key:       q.Get("key"),
		format:    q.Get("format"),
		prefix:    q.Get("prefix"),
		kvVersion: 2,
		pollMs:    5000,
	}
	if cs.format == "" {
		cs.format = "properties"
	}

	if v := q.Get("kv-version"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || (n != 1 && n != 2) {
			return configSource{}, errutil.Explain(nil, "kv-version must be 1 or 2, got %q", v)
		}
		cs.kvVersion = n
	}
	if v := q.Get("poll-ms"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return configSource{}, errutil.Explain(nil, "invalid poll-ms in %q", source)
		}
		cs.pollMs = n
	}

	token, err := resolveToken(q)
	if err != nil {
		return configSource{}, err
	}
	cs.token = token
	return cs, nil
}

// resolveToken locates the Vault token, preferring out-of-band sources so the
// token does not need to live in a configuration file. Order: the token query
// parameter (discouraged, but allowed for local demos), the VAULT_TOKEN
// environment variable, then a token file named by the token-file query
// parameter or the VAULT_TOKEN_FILE environment variable.
func resolveToken(q url.Values) (string, error) {
	if v := q.Get("token"); v != "" {
		return v, nil
	}
	if v := os.Getenv("VAULT_TOKEN"); v != "" {
		return v, nil
	}
	path := q.Get("token-file")
	if path == "" {
		path = os.Getenv("VAULT_TOKEN_FILE")
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", errutil.Explain(err, "read vault token file %q failed", path)
		}
		if t := strings.TrimSpace(string(b)); t != "" {
			return t, nil
		}
	}
	return "", errutil.Explain(nil, "no vault token found (set VAULT_TOKEN, VAULT_TOKEN_FILE, or the token query parameter)")
}

// clientCache reuses one Vault client per (address, namespace, token) tuple.
// loadVaultConfig runs at startup and again on every RefreshProperties call, so
// caching avoids rebuilding a client on each refresh.
var (
	clientMu    sync.Mutex
	clientCache = map[string]*api.Client{}
	// listened tracks (client-key, mount, path) tuples that already have a
	// polling watcher, so repeated Load calls do not start duplicates.
	listened = map[string]struct{}{}
	// loadedFP records, per watched tuple, the fingerprint of the secret data
	// as of the last time loadVaultConfig read it. The polling watcher compares
	// against this shared value rather than a private baseline, so a change is
	// detected relative to what the application actually loaded — even when the
	// secret is created after startup (optional import) between the startup read
	// and the first poll.
	loadedFP = map[string]string{}
)

// watchKey identifies a watched secret across the client cache, mount and path.
func watchKey(key string, cs configSource) string {
	return key + "|" + cs.mount + "|" + cs.path
}

// setLoadedFP records the fingerprint of the data most recently loaded for lk.
func setLoadedFP(lk, fp string) {
	clientMu.Lock()
	loadedFP[lk] = fp
	clientMu.Unlock()
}

// getLoadedFP returns the last-loaded fingerprint for lk.
func getLoadedFP(lk string) string {
	clientMu.Lock()
	defer clientMu.Unlock()
	return loadedFP[lk]
}

// clientFor returns a cached client for the source, creating one if necessary.
// It also returns the cache key so listener registration can dedupe.
func clientFor(cs configSource) (*api.Client, string, error) {
	key := cs.address + "|" + cs.namespace + "|" + cs.token

	clientMu.Lock()
	defer clientMu.Unlock()

	if cli, ok := clientCache[key]; ok {
		return cli, key, nil
	}

	cfg := api.DefaultConfig()
	cfg.Address = cs.address
	cli, err := api.NewClient(cfg)
	if err != nil {
		return nil, "", errutil.Explain(err, "create vault client for %s failed", cs.address)
	}
	cli.SetToken(cs.token)
	if cs.namespace != "" {
		cli.SetNamespace(cs.namespace)
	}
	clientCache[key] = cli
	return cli, key, nil
}

// loadVaultConfig implements conf/provider.Provider. It reads a KV secret,
// turns it into a flattened property map, and installs a polling watcher that
// triggers an application property refresh when the secret changes.
func loadVaultConfig(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}

	cli, key, err := clientFor(cs)
	if err != nil {
		return nil, err
	}

	// Register the watcher before reading so that hot-reload works even when
	// the secret does not exist yet: a later write triggers a refresh that
	// re-runs this provider and picks up the new value.
	registerWatch(cli, key, cs)

	data, err := readSecret(cli, cs)
	if err != nil {
		if optional {
			return nil, nil
		}
		return nil, err
	}
	// Record the fingerprint of what we just loaded so the polling watcher
	// detects changes relative to this state rather than an independent seed.
	setLoadedFP(watchKey(key, cs), fingerprint(data))
	if data == nil {
		if optional {
			return nil, nil
		}
		return nil, errutil.Explain(nil, "vault secret %s/%s not found", cs.mount, cs.path)
	}
	return toProperties(cs, data)
}

// readSecret fetches the raw KV data map for the source, or nil when the secret
// does not exist.
func readSecret(cli *api.Client, cs configSource) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		secret *api.KVSecret
		err    error
	)
	if cs.kvVersion == 2 {
		secret, err = cli.KVv2(cs.mount).Get(ctx, cs.path)
	} else {
		secret, err = cli.KVv1(cs.mount).Get(ctx, cs.path)
	}
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, errutil.Explain(err, "read vault secret %s/%s failed", cs.mount, cs.path)
	}
	if secret == nil {
		return nil, nil
	}
	return secret.Data, nil
}

// isNotFound reports whether the error indicates a missing secret rather than a
// transport or auth failure.
func isNotFound(err error) bool {
	var respErr *api.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return strings.Contains(err.Error(), "404")
}

// toProperties turns the raw KV data map into a flattened property map. In
// whole-secret mode every field becomes a property; in single-field mode
// (?key=...) the named field is parsed as a document in the declared format.
func toProperties(cs configSource, data map[string]any) (map[string]string, error) {
	if cs.key != "" {
		raw, ok := data[cs.key]
		if !ok {
			return nil, errutil.Explain(nil, "vault secret %s/%s has no field %q", cs.mount, cs.path, cs.key)
		}
		s, ok := raw.(string)
		if !ok {
			return nil, errutil.Explain(nil, "vault field %q is not a string document", cs.key)
		}
		r, ok := contentReaders[cs.format]
		if !ok {
			return nil, errutil.Explain(nil, "unsupported vault config format %q", cs.format)
		}
		m, err := r([]byte(s))
		if err != nil {
			return nil, errutil.Explain(err, "parse vault field %q as %s failed", cs.key, cs.format)
		}
		return withPrefix(cs.prefix, flatten.Flatten(m)), nil
	}
	return withPrefix(cs.prefix, flatten.Flatten(data)), nil
}

// withPrefix prepends prefix (plus a dot) to every key when prefix is non-empty.
func withPrefix(prefix string, m map[string]string) map[string]string {
	if prefix == "" {
		return m
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[prefix+"."+k] = v
	}
	return out
}

// registerWatch spawns a background goroutine that polls the secret and fires
// triggerRefresh whenever its content changes. Deduplicated across repeated
// Load calls.
func registerWatch(cli *api.Client, key string, cs configSource) {
	lk := watchKey(key, cs)

	clientMu.Lock()
	if _, ok := listened[lk]; ok {
		clientMu.Unlock()
		return
	}
	listened[lk] = struct{}{}
	clientMu.Unlock()

	go watchLoop(cli, cs, lk)
}

// watchLoop polls the secret every cs.pollMs and triggers a refresh whenever the
// polled content fingerprint differs from the one loadVaultConfig last loaded
// (see loadedFP). triggerRefresh re-runs loadVaultConfig, which updates that
// shared fingerprint, so a change fires exactly once. Startup does not fire a
// spurious refresh because the startup load already seeded the fingerprint.
func watchLoop(cli *api.Client, cs configSource, lk string) {
	interval := time.Duration(cs.pollMs) * time.Millisecond
	for {
		time.Sleep(interval)
		data, err := readSecret(cli, cs)
		if err != nil {
			continue // transient: retry on next tick
		}
		if fingerprint(data) != getLoadedFP(lk) {
			triggerRefresh()
		}
	}
}

// fingerprint produces a stable string representation of a KV data map for
// change detection. A nil map (missing secret) has a distinct fingerprint from
// an empty one.
func fingerprint(data map[string]any) string {
	if data == nil {
		return "<nil>"
	}
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(b)
}
