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

// Package StarterConfigVault integrates HashiCorp Vault as a remote
// configuration center for Go-Spring. Blank-importing this package registers a
// "vault" config provider that can be consumed via spring.app.imports, together
// with the bridge that wires secret changes into the application-wide property
// refresh for live hot-reload.
//
// This starter covers the config-center role only: it reads a KV secret and
// exposes its fields as application properties. A Vault Agent / CSI-mounted
// secret file is read with starter-config-file instead; this starter talks to
// the Vault API directly.
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
	"time"

	"github.com/hashicorp/vault/api"
	"go-spring.org/spring/conf"
	confjson "go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register the vault controller as both a root bean (so the IoC container
	// injects its PropertiesRefresher via autowire) and the "vault" config
	// provider (so Load calls go through its method). Before wiring,
	// TriggerRefresh is a harmless no-op — the startup load already captured
	// the initial config.
	gs.Provide(vaultController).Export(gs.As[gs.Rooter]())

	// Register "vault" as a remote configuration provider. The provider is
	// the global controller's Load method, so the same object that holds the
	// PropertiesRefresher (injected via autowire) also serves config loads.
	conf.RegisterProvider("vault", vaultController.Load)
}

// vaultController is the global singleton. It is ONLY referenced in init
// functions. All other code operates on the
// receiver without touching this global.
var vaultController = &vaultCtrl{}

// vaultCtrl is the single object that owns the full lifecycle of vault
// configuration: loading secrets, polling for changes, and triggering
// property refresh.
type vaultCtrl struct {
	Refresher *gs.PropertiesRefresher `autowire:""`

	mu       sync.Mutex
	clients  map[string]*api.Client
	listened map[string]struct{}
	loadedFP map[string]string // fingerprint of last loaded data
}

// TriggerRefresh is called by the polling watchers when a secret's content
// fingerprint changes. Before the IoC container wires the controller, this
// is a no-op — the initial config load already captured the state.
func (c *vaultCtrl) TriggerRefresh() {
	if c.Refresher != nil {
		_ = c.Refresher.RefreshProperties()
	}
}

// contentReader parses raw configuration bytes into a nested map.
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

// configSource holds the parsed components of a vault provider source string.
type configSource struct {
	address   string
	namespace string
	token     string
	mount     string
	path      string
	kvVersion int
	key       string
	format    string
	prefix    string
	pollMs    int
}

// parseSource parses a provider source of the form
// <host>:<port>/<mount>/<path>?kv-version=..&token=..&namespace=..&scheme=..&key=..&format=..&prefix=..&poll-ms=..
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

// resolveToken locates the Vault token, preferring out-of-band sources.
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

// clientKey builds a cache key for a client.
func clientKey(cs configSource) string {
	return cs.address + "|" + cs.namespace + "|" + cs.token
}

// clientFor returns a cached client for the source, creating one if necessary.
func (c *vaultCtrl) clientFor(cs configSource) (*api.Client, error) {
	key := clientKey(cs)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.clients == nil {
		c.clients = map[string]*api.Client{}
	}
	if cli, ok := c.clients[key]; ok {
		return cli, nil
	}

	cfg := api.DefaultConfig()
	cfg.Address = cs.address
	cli, err := api.NewClient(cfg)
	if err != nil {
		return nil, errutil.Explain(err, "create vault client for %s failed", cs.address)
	}
	cli.SetToken(cs.token)
	if cs.namespace != "" {
		cli.SetNamespace(cs.namespace)
	}
	c.clients[key] = cli
	return cli, nil
}

// watchKey identifies a watched secret.
func watchKey(cs configSource) string {
	return clientKey(cs) + "|" + cs.mount + "|" + cs.path
}

// Load implements conf/provider.Provider. It reads a KV secret, turns it into
// a flattened property map, and installs a polling watcher that triggers an
// application property refresh when the secret changes.
func (c *vaultCtrl) Load(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}

	cli, err := c.clientFor(cs)
	if err != nil {
		return nil, err
	}

	c.registerWatch(cli, cs)

	data, err := c.readSecret(cli, cs)
	if err != nil {
		if optional {
			return nil, nil
		}
		return nil, err
	}

	lk := watchKey(cs)
	c.mu.Lock()
	if c.loadedFP == nil {
		c.loadedFP = map[string]string{}
	}
	c.loadedFP[lk] = fingerprint(data)
	c.mu.Unlock()

	if data == nil {
		if optional {
			return nil, nil
		}
		return nil, errutil.Explain(nil, "vault secret %s/%s not found", cs.mount, cs.path)
	}
	return toProperties(cs, data)
}

// readSecret fetches the raw KV data map for the source.
func (c *vaultCtrl) readSecret(cli *api.Client, cs configSource) (map[string]any, error) {
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

// isNotFound reports whether the error indicates a missing secret.
func isNotFound(err error) bool {
	var respErr *api.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return strings.Contains(err.Error(), "404")
}

// toProperties turns the raw KV data map into a flattened property map.
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

// registerWatch spawns a background goroutine that polls the secret.
func (c *vaultCtrl) registerWatch(cli *api.Client, cs configSource) {
	lk := watchKey(cs)

	c.mu.Lock()
	if c.listened == nil {
		c.listened = map[string]struct{}{}
	}
	if _, ok := c.listened[lk]; ok {
		c.mu.Unlock()
		return
	}
	c.listened[lk] = struct{}{}
	c.mu.Unlock()

	go c.watchLoop(cli, cs, lk)
}

// watchLoop polls the secret and triggers a refresh whenever the content
// fingerprint differs from the last loaded value.
func (c *vaultCtrl) watchLoop(cli *api.Client, cs configSource, lk string) {
	interval := time.Duration(cs.pollMs) * time.Millisecond
	for {
		time.Sleep(interval)
		data, err := c.readSecret(cli, cs)
		if err != nil {
			continue
		}
		c.mu.Lock()
		fp := c.loadedFP[lk]
		c.mu.Unlock()
		if fingerprint(data) != fp {
			c.TriggerRefresh()
		}
	}
}

// fingerprint produces a stable string representation of a KV data map.
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
