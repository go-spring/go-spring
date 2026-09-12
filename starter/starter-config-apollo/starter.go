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
//
// This starter covers the config-center role only.
package StarterConfigApollo

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/apolloconfig/agollo/v4"
	agconfig "github.com/apolloconfig/agollo/v4/env/config"
	agstorage "github.com/apolloconfig/agollo/v4/storage"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// agolloChangeEvent / agolloFullChangeEvent alias agollo's event types so the
// listener implementation stays readable without importing the storage package
// name everywhere.
type agolloChangeEvent = agstorage.ChangeEvent
type agolloFullChangeEvent = agstorage.FullChangeEvent

// apolloClient is the slice of agollo.Client the controller actually uses.
// Narrowing to an interface keeps the type fakeable in tests without
// standing up the sixteen-method agollo.Client surface (whose Config cannot
// even be constructed outside agollo).
type apolloClient interface {
	// GetConfigContent returns the raw namespace content, or "" when the
	// namespace has not been synced yet.
	GetConfigContent(namespace string) string
	AddChangeListener(listener agstorage.ChangeListener)
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

func init() {
	conf.RegisterProvider("apollo", (&apolloCtrl{}).Load)
}

// apolloCtrl owns the full lifecycle of apollo configuration: loading
// namespaces, listening for changes, and triggering property refresh.
type apolloCtrl struct {
	mu       sync.Mutex
	clients  map[string]apolloClient
	listened map[string]struct{}
}

// TriggerRefresh is called by the config listener when a watched namespace
// changes. Before the app has started, gs.RefreshProperties returns an
// error and the change is dropped — the initial load already captured the
// state.
func (c *apolloCtrl) TriggerRefresh() {
	_ = gs.RefreshProperties()
}

// apolloSource holds the parsed components of an apollo provider source.
type apolloSource struct {
	server    string
	namespace string
	appID     string
	cluster   string
	secret    string
	format    string
}

// parseSource parses a source of the form
// <host>:<port>/<namespace>?appId=..&cluster=..&secret=..&format=..
func parseSource(source string) (apolloSource, error) {
	u, err := url.Parse("apollo://" + source)
	if err != nil {
		return apolloSource{}, errutil.Explain(err, "invalid apollo source %q", source)
	}
	if u.Host == "" {
		return apolloSource{}, errutil.Explain(nil, "missing apollo server address in %q", source)
	}
	ns := strings.TrimPrefix(u.Path, "/")
	if ns == "" {
		return apolloSource{}, errutil.Explain(nil, "missing namespace in %q", source)
	}
	q := u.Query()
	cs := apolloSource{
		server:    u.Host,
		namespace: ns,
		appID:     q.Get("appId"),
		cluster:   q.Get("cluster"),
		secret:    q.Get("secret"),
		format:    q.Get("format"),
	}
	if cs.appID == "" {
		return apolloSource{}, errutil.Explain(nil, "missing appId in %q", source)
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
func clientKey(cs apolloSource) string {
	return cs.server + "|" + cs.appID + "|" + cs.cluster + "|" + cs.secret + "|" + cs.namespace
}

// clientFor returns a cached agollo Client, creating one if necessary.
func (c *apolloCtrl) clientFor(cs apolloSource) (apolloClient, error) {
	key := clientKey(cs)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.clients == nil {
		c.clients = map[string]apolloClient{}
	}
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
		}, nil
	})
	if err != nil {
		return nil, errutil.Explain(err, "create apollo client for %s failed", cs.server)
	}
	c.clients[key] = agolloClientAdapter{Client: cli}
	return c.clients[key], nil
}

// Load implements conf/provider.Provider. It fetches the namespace content,
// parses it according to the declared format, and installs a change listener
// that triggers an application property refresh.
func (c *apolloCtrl) Load(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		log.Errorf(context.Background(), log.TagAppDef, "parse source %q failed: %v", source, err)
		return nil, err
	}

	cli, err := c.clientFor(cs)
	if err != nil {
		log.Errorf(context.Background(), log.TagAppDef, "create apollo client for appId=%s failed: %v", cs.appID, err)
		return nil, err
	}

	// Install the listener BEFORE the fetch so a later change is never missed.
	c.registerListener(cli, cs)

	content := cli.GetConfigContent(cs.namespace)
	if content == "" {
		if optional {
			log.Warnf(context.Background(), log.TagAppDef, "optional apollo namespace %s is empty (skipped)", cs.namespace)
			return nil, nil
		}
		return nil, errutil.Explain(nil, "apollo namespace %s is empty", cs.namespace)
	}

	m, err := reader.Read(cs.format, []byte(content))
	if err != nil {
		return nil, errutil.Explain(err, "parse apollo namespace %s as %s failed", cs.namespace, cs.format)
	}
	log.Infof(context.Background(), log.TagAppDef, "loaded apollo namespace %s keys=%d", cs.namespace, len(m))
	return flatten.Flatten(m), nil
}

// registerListener installs an agollo change listener for the client,
// deduplicated across repeated Load calls.
func (c *apolloCtrl) registerListener(cli apolloClient, cs apolloSource) {
	lk := clientKey(cs)

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

	cli.AddChangeListener(&apolloListener{ctrl: c})
}

// apolloListener adapts agollo's ChangeListener to the refresh trigger.
type apolloListener struct {
	ctrl *apolloCtrl
}

func (l *apolloListener) OnChange(*agolloChangeEvent) {
	l.ctrl.TriggerRefresh()
}

func (l *apolloListener) OnNewestChange(*agolloFullChangeEvent) {
	l.ctrl.TriggerRefresh()
}
