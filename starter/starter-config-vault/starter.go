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
// "vault" config provider that can be consumed via spring.config.import, together
// with the bridge that wires secret changes into the application-wide property
// refresh for live hot-reload.
package StarterConfigVault

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

var starterTag = log.RegisterAppTag("config", "vault")

func init() {
	conf.RegisterProvider("vault", newVaultCtrl())
}

// vaultKV is one KV read on a mount, and vaultAPI is the slice of the Vault
// client this starter consumes. They exist so tests can fake the Vault backend
// without a live server, mirroring consul's kvAPI.
//
// The adapter is needed because *api.Client's KVv1/KVv2 return concrete types
// and Go has no covariant returns, so it cannot satisfy these interfaces as
// written.
type vaultKV interface {
	Get(ctx context.Context, path string) (*api.KVSecret, error)
}

type vaultAPI interface {
	KVv1(mount string) vaultKV
	KVv2(mount string) vaultKV
}

type realVault struct{ c *api.Client }

func (r realVault) KVv1(mount string) vaultKV { return r.c.KVv1(mount) }
func (r realVault) KVv2(mount string) vaultKV { return r.c.KVv2(mount) }

// vaultCtrl owns the full lifecycle of vault configuration: loading secrets
// through its client cache, and polling for changes through its watchCore,
// which also keeps the loaded-fingerprint baseline. The two sides keep
// separate mutexes: the client cache never contends with the pollers.
type vaultCtrl struct {
	watch    watchCore
	clientMu sync.Mutex
	clients  map[string]vaultAPI
}

func newVaultCtrl() *vaultCtrl {
	return &vaultCtrl{
		clients: map[string]vaultAPI{},
		watch:   newWatchCore(),
	}
}

// Close stops every poll goroutine. It implements provider.Provider.
func (c *vaultCtrl) Close() {
	c.watch.stop()
	c.clientMu.Lock()
	c.clients = map[string]vaultAPI{}
	c.clientMu.Unlock()
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
	timeoutMs int
}

// redactSource masks the token query parameter of a source string so
// the remainder is safe to print. Everything user-visible — parse errors, log
// fields — must carry the redacted form; the raw form never reaches output.
func redactSource(source string) string {
	i := strings.Index(source, "?")
	if i < 0 {
		return source
	}
	q := strings.Split(source[i+1:], "&")
	kept := q[:0]
	for _, kv := range q {
		if strings.HasPrefix(kv, "token=") {
			kv = "token=***"
		}
		kept = append(kept, kv)
	}
	return source[:i+1] + strings.Join(kept, "&")
}

// parseSource parses a provider source of the form
// <host>:<port>/<mount>/<path>?kv-version=..&token=..&namespace=..&scheme=..&key=..&format=..&prefix=..&poll-ms=..&timeout-ms=..
func parseSource(source string) (configSource, error) {
	u, err := url.Parse("vault://" + source)
	if err != nil {
		return configSource{}, errutil.Explain(err, "invalid vault source %q", redactSource(source))
	}
	if u.Host == "" {
		return configSource{}, errutil.Explain(nil, "missing vault server address in %q", redactSource(source))
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
		timeoutMs: 5000,
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
			return configSource{}, errutil.Explain(nil, "invalid poll-ms in %q", redactSource(source))
		}
		cs.pollMs = n
	}
	if v := q.Get("timeout-ms"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return configSource{}, errutil.Explain(nil, "invalid timeout-ms in %q", redactSource(source))
		}
		cs.timeoutMs = n
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
// Its log line takes the source fields from ctx, which Load has already stamped.
func (c *vaultCtrl) clientFor(ctx context.Context, cs configSource) (vaultAPI, error) {
	key := clientKey(cs)

	c.clientMu.Lock()
	defer c.clientMu.Unlock()

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
	log.Infof(ctx, starterTag, "create vault client success")
	v := realVault{cli}
	c.clients[key] = v
	return v, nil
}

// Load implements conf/provider.Provider. It reads a KV secret, turns it into
// a flattened property map, and installs a polling watcher that triggers an
// application property refresh when the secret changes.
func (c *vaultCtrl) Load(ctx context.Context, optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		log.Error(ctx, starterTag, err,
			log.String("source", redactSource(source)),
			log.Msg("parse vault source failed"))
		return nil, err
	}

	// The source's identity rides on the context from here on: every event this
	// load and its helpers print carries it without repeating it at each call
	// site.
	ctx = log.WithFields(ctx,
		log.String("address", cs.address),
		log.String("mount", cs.mount),
		log.String("format", cs.format),
		log.Int("kv_version", cs.kvVersion),
		log.String("key", cs.key),
		log.String("path", cs.path))

	log.Debugf(ctx, starterTag, "loading vault config")

	cli, err := c.clientFor(ctx, cs)
	if err != nil {
		log.Errorf(ctx, starterTag, err, "create vault client failed")
		return nil, err
	}

	c.watch.registerWatch(cli, cs, c.readSecret)
	return c.loadFromClient(ctx, cli, cs, optional)
}

// loadFromClient reads the secret once, applies the optional/not-found rules,
// and parses it into flattened properties. The caller installs the watcher
// before calling it, so a change landing right after the read is not missed.
func (c *vaultCtrl) loadFromClient(ctx context.Context, cli vaultAPI, cs configSource, optional bool) (map[string]string, error) {
	data, err := c.readSecret(ctx, cli, cs)
	if err != nil {
		if optional {
			log.Warn(ctx, starterTag,
				log.Err(err),
				log.Msg("skip optional config read secret failed"))
			return nil, nil
		}
		return nil, err
	}

	c.watch.updateBaseline(cs, fingerprint(data))

	if data == nil {
		if optional {
			log.Warnf(ctx, starterTag, "skip optional config secret not found")
			return nil, nil
		}
		err := errutil.Explain(nil, "vault secret %s/%s not found", cs.mount, cs.path)
		log.Errorf(ctx, starterTag, err, "vault secret not found")
		return nil, err
	}

	props, err := toProperties(cs, data)
	if err != nil {
		return nil, err
	}
	log.Infof(ctx, starterTag, "load vault config success")
	return props, nil
}

// readSecret fetches the raw KV data map for the source. The source's
// timeout-ms bounds the read and is honored on shutdown: cancelling the parent
// context aborts an in-flight poll.
func (c *vaultCtrl) readSecret(ctx context.Context, cli vaultAPI, cs configSource) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cs.timeoutMs)*time.Millisecond)
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
		m, err := reader.Read(cs.format, []byte(s))
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
