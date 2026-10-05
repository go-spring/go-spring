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

// Package StarterConfigApollo integrates Apollo as a remote configuration
// center for Go-Spring. Blank-importing this package registers an "apollo"
// config provider consumable via spring.config.import, together with the
// bridge that wires remote config changes into the application-wide property
// refresh for live hot-reload.
package StarterConfigApollo

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/apolloconfig/agollo/v4"
	agconfig "github.com/apolloconfig/agollo/v4/env/config"
	agstorage "github.com/apolloconfig/agollo/v4/storage"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

var starterTag = log.RegisterAppTag("config", "apollo")

func init() {
	conf.RegisterProvider("apollo", newApolloCtrl())
}

// apolloClient is the subset of agollo.Client the controller uses.
type apolloClient interface {
	// GetConfigContent returns the raw namespace content, or "" when the
	// namespace has not been synced yet.
	GetConfigContent(namespace string) string
	// AddChangeListener subscribes the listener to namespace change events,
	// delivered from the client's long-polling goroutines.
	AddChangeListener(listener agstorage.ChangeListener)
	// Close stops the client's long polling, which is what ends the change
	// callbacks at shutdown.
	Close()
}

// agolloClientAdapter adapts a real agollo.Client to apolloClient.
type agolloClientAdapter struct {
	agollo.Client
}

func (a agolloClientAdapter) GetConfigContent(namespace string) string {
	if cfg := a.Client.GetConfig(namespace); cfg != nil {
		return cfg.GetContent()
	}
	return ""
}

// apolloCtrl owns the full lifecycle of apollo configuration: loading
// namespaces, listening for changes, and triggering property refresh.
type apolloCtrl struct {
	mu       sync.Mutex
	clients  map[string]apolloClient
	listened map[string]struct{}
}

func newApolloCtrl() *apolloCtrl {
	return &apolloCtrl{
		clients:  map[string]apolloClient{},
		listened: map[string]struct{}{},
	}
}

// Close stops the long polling of every client and drops the caches.
func (c *apolloCtrl) Close() {
	c.mu.Lock()
	clients := c.clients
	c.clients = map[string]apolloClient{}
	c.listened = map[string]struct{}{}
	c.mu.Unlock()

	for _, cli := range clients {
		cli.Close()
	}
	if n := len(clients); n > 0 {
		log.Infof(context.Background(), starterTag, "apollo client(s) closed")
	}
}

// configSource holds the parsed components of an apollo provider source.
type configSource struct {
	server    string
	namespace string
	appID     string
	cluster   string
	secret    string
	format    string
}

// redactSource masks the secret query parameter of a source string so the
// remainder is safe to print. Everything user-visible — parse errors, log
// fields — must carry the redacted form; the raw form never reaches output.
func redactSource(source string) string {
	i := strings.Index(source, "?")
	if i < 0 {
		return source
	}
	q := strings.Split(source[i+1:], "&")
	kept := q[:0]
	for _, kv := range q {
		if strings.HasPrefix(kv, "secret=") {
			kv = "secret=***"
		}
		kept = append(kept, kv)
	}
	return source[:i+1] + strings.Join(kept, "&")
}

// parseSource parses a source of the form
// <host>:<port>/<namespace>?appId=..&cluster=..&secret=..&format=..
func parseSource(source string) (configSource, error) {
	u, err := url.Parse("apollo://" + source)
	if err != nil {
		return configSource{}, errutil.Explain(err, "invalid apollo source %q", redactSource(source))
	}
	if u.Host == "" {
		return configSource{}, errutil.Explain(nil, "missing apollo server address in %q", redactSource(source))
	}
	ns := strings.TrimPrefix(u.Path, "/")
	if ns == "" {
		return configSource{}, errutil.Explain(nil, "missing namespace in %q", redactSource(source))
	}
	q := u.Query()
	cs := configSource{
		server:    u.Host,
		namespace: ns,
		appID:     q.Get("appId"),
		cluster:   q.Get("cluster"),
		secret:    q.Get("secret"),
		format:    q.Get("format"),
	}
	if cs.appID == "" {
		return configSource{}, errutil.Explain(nil, "missing appId in %q", redactSource(source))
	}
	if cs.cluster == "" {
		cs.cluster = "default"
	}
	if cs.format == "" {
		if ext := strings.TrimPrefix(filepath.Ext(ns), "."); ext != "" {
			cs.format = ext
		} else {
			cs.format = "properties"
		}
	}
	return cs, nil
}

// clientKey builds a cache key for a client: one agollo Client per
// (server, appId, cluster, secret, namespace), so each import's namespace is
// its own synced config.
func clientKey(cs configSource) string {
	return cs.server + "|" + cs.appID + "|" + cs.cluster + "|" + cs.secret + "|" + cs.namespace
}

// clientFor returns a cached agollo Client, creating one if necessary. The
// namespace identity fields come from ctx, which Load has already stamped.
func (c *apolloCtrl) clientFor(ctx context.Context, cs configSource) (apolloClient, error) {
	key := clientKey(cs)

	c.mu.Lock()
	defer c.mu.Unlock()

	if cli, ok := c.clients[key]; ok {
		return cli, nil
	}

	cli, err := agollo.StartWithConfig(func() (*agconfig.AppConfig, error) {
		return &agconfig.AppConfig{
			AppID:          cs.appID,
			Cluster:        cs.cluster,
			NamespaceName:  cs.namespace,
			IP:             "http://" + cs.server,
			Secret:         cs.secret,
			IsBackupConfig: false,
			MustStart:      true,
		}, nil
	})
	if err != nil {
		return nil, errutil.Explain(err, "create apollo client for %s failed", cs.server)
	}
	log.Infof(ctx, starterTag, "create apollo client success")
	c.clients[key] = agolloClientAdapter{Client: cli}
	return c.clients[key], nil
}

// Load implements conf/provider.Provider. It fetches the namespace content,
// parses it according to the declared format, and installs a change listener
// that triggers an application property refresh.
func (c *apolloCtrl) Load(ctx context.Context, optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		log.Error(ctx, starterTag, err, log.String("source", redactSource(source)), log.Msg("parse apollo source failed"))
		return nil, err
	}

	// Carry the namespace's identity on the context so every event below logs
	// it without repeating these fields at each call site.
	ctx = log.WithFields(ctx,
		log.String("namespace", cs.namespace),
		log.String("server", cs.server),
		log.String("app_id", cs.appID),
		log.String("cluster", cs.cluster),
		log.String("format", cs.format),
	)

	cli, err := c.clientFor(ctx, cs)
	if err != nil {
		// The first sync failed (server unreachable or namespace missing).
		// An optional import skips the namespace instead of failing startup,
		// but the error is still logged — an unreachable config server is
		// never silently ignored.
		if optional {
			log.Warn(ctx, starterTag, log.Err(err),
				log.Msg("skip optional apollo import when create apollo client failed"))
			return nil, nil
		}
		log.Errorf(ctx, starterTag, err, "create apollo client failed")
		return nil, err
	}

	log.Debugf(ctx, starterTag, "loading apollo namespace")

	// Install the listener BEFORE the fetch so a later change is never missed.
	c.registerListener(ctx, cli, cs)
	return loadFromClient(ctx, cli, cs, optional)
}

// loadFromClient reads the namespace once, applies the optional/empty rules, and
// parses it into flattened properties. The caller installs the listener before
// calling it, so a change landing right after the read is not missed.
func loadFromClient(ctx context.Context, cli apolloClient, cs configSource, optional bool) (map[string]string, error) {
	content := cli.GetConfigContent(cs.namespace)
	if content == "" {
		if optional {
			log.Warnf(ctx, starterTag, "skip empty optional apollo namespace")
			return nil, nil
		}
		return nil, errutil.Explain(nil, "apollo namespace %s is empty", cs.namespace)
	}

	m, err := reader.Read(cs.format, []byte(content))
	if err != nil {
		log.Errorf(ctx, starterTag, err, "parse apollo namespace failed")
		return nil, errutil.Explain(err, "parse apollo namespace %s as %s failed", cs.namespace, cs.format)
	}
	log.Infof(ctx, starterTag, "load apollo namespace success")
	return flatten.Flatten(m), nil
}

// registerListener installs an agollo change listener for the client,
// deduplicated across repeated Load calls.
func (c *apolloCtrl) registerListener(ctx context.Context, cli apolloClient, cs configSource) {
	lk := clientKey(cs)

	c.mu.Lock()
	if _, ok := c.listened[lk]; ok {
		c.mu.Unlock()
		return
	}
	c.listened[lk] = struct{}{}
	c.mu.Unlock()

	log.Infof(ctx, starterTag, "watching apollo namespace for changes")
	cli.AddChangeListener(&apolloListener{})
}

// apolloListener adapts agollo's ChangeListener to the refresh trigger.
type apolloListener struct{}

func (l *apolloListener) OnChange(ev *agstorage.ChangeEvent) {
	// The refresh id names the namespace this round is about; agollo hands
	// the listener no change revision, so the namespace is the whole native
	// identity available.
	refreshID := fmt.Sprintf("apollo:%s", ev.Namespace)
	log.Info(context.Background(), starterTag,
		log.String("refresh_id", refreshID),
		log.Msg("apollo namespace changed, triggering refresh"))
	_ = observability.RefreshConf(refreshID, func() error {
		return gs.RefreshProperties(refreshID)
	})
}

// OnNewestChange must exist to satisfy agstorage.ChangeListener but is a
// deliberate no-op: agollo dispatches it unconditionally alongside OnChange
// on every config fetch, so refreshing here would double-fire (and race) a
// full property refresh on each real change.
func (l *apolloListener) OnNewestChange(ev *agstorage.FullChangeEvent) {}
