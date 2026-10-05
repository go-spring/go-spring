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

// Package StarterConfigEtcd integrates etcd as a remote configuration
// center for Go-Spring. Blank-importing this package registers an "etcd"
// config provider that can be consumed via spring.config.import, together with
// the bridge that wires remote config changes into the application-wide
// property refresh for live hot-reload.
package StarterConfigEtcd

import (
	"context"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var starterTag = log.RegisterAppTag("config", "etcd")

func init() {
	conf.RegisterProvider("etcd", newEtcdCtrl())
}

// etcdAPI is the slice of the etcd client this starter consumes. It exists so
// tests can fake the etcd backend without a live server, mirroring consul's
// kvAPI.
type etcdAPI interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
	Close() error
}

// etcdCtrl owns the full lifecycle of etcd configuration: loading keys
// through its client cache, and watching for changes through the embedded
// watchCore. Its clients are bootstrap infrastructure: they exist
// before the container does and therefore deliberately opt out of
// observability and governance wiring — don't "fix" that here; consumers
// that need instrumented clients (e.g. the governance source) build their
// own.
type etcdCtrl struct {
	watch    watchCore
	clientMu sync.Mutex
	clients  map[string]etcdAPI
}

func newEtcdCtrl() *etcdCtrl {
	return &etcdCtrl{
		clients: map[string]etcdAPI{},
		watch:   newWatchCore(),
	}
}

// Close stops every watch goroutine and closes every client.
func (c *etcdCtrl) Close() {
	c.watch.stop()

	c.clientMu.Lock()
	clients := c.clients
	c.clients = map[string]etcdAPI{}
	c.clientMu.Unlock()

	for _, cli := range clients {
		if err := cli.Close(); err != nil {
			log.Warn(context.Background(), starterTag,
				log.Err(err),
				log.Msg("closing etcd client failed"))
		}
	}

	if len(clients) > 0 {
		log.Infof(context.Background(), starterTag, "etcd clients closed")
	}
}

// configSource holds the parsed components of an etcd provider source string.
type configSource struct {
	endpoint    string
	key         string
	username    string
	password    string
	dialTimeout time.Duration
	format      string
	retryMs     int
}

// redactSource masks the password query parameter of a source string so
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
		if strings.HasPrefix(kv, "password=") {
			kv = "password=***"
		}
		kept = append(kept, kv)
	}
	return source[:i+1] + strings.Join(kept, "&")
}

// parseSource parses a provider source of the form
// <host>:<port>/<key>?format=..&username=..&password=..&dial-timeout=..&retry-ms=..
func parseSource(source string) (configSource, error) {
	u, err := url.Parse("etcd://" + source)
	if err != nil {
		return configSource{}, errutil.Explain(err, "invalid etcd source %q", redactSource(source))
	}
	if u.Host == "" {
		return configSource{}, errutil.Explain(nil, "missing etcd server address in %q", redactSource(source))
	}
	key := strings.TrimPrefix(u.Path, "/")
	if key == "" {
		return configSource{}, errutil.Explain(nil, "missing etcd key in %q", redactSource(source))
	}

	q := u.Query()
	cs := configSource{
		endpoint: u.Host,
		key:      key,
		username: q.Get("username"),
		password: q.Get("password"),
		format:   q.Get("format"),
	}
	if cs.format == "" {
		if ext := strings.TrimPrefix(filepath.Ext(key), "."); ext != "" {
			cs.format = ext
		} else {
			cs.format = "properties"
		}
	}
	cs.dialTimeout = 5 * time.Second
	if v := q.Get("dial-timeout"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return configSource{}, errutil.Explain(err, "invalid dial-timeout in %q", redactSource(source))
		}
		cs.dialTimeout = d
	}
	cs.retryMs = 5000
	if v := q.Get("retry-ms"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return configSource{}, errutil.Explain(nil, "invalid retry-ms in %q", redactSource(source))
		}
		cs.retryMs = n
	}
	return cs, nil
}

// clientKey builds a cache key for a client.
func clientKey(cs configSource) string {
	return cs.endpoint + "|" + cs.username + "|" + cs.password
}

// clientFor returns a cached client for the source, creating one if necessary.
// Its log line takes the source fields from ctx, which Load has already stamped.
func (c *etcdCtrl) clientFor(ctx context.Context, cs configSource) (etcdAPI, error) {
	key := clientKey(cs)

	c.clientMu.Lock()
	defer c.clientMu.Unlock()

	if cli, ok := c.clients[key]; ok {
		return cli, nil
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{cs.endpoint},
		Username:    cs.username,
		Password:    cs.password,
		DialTimeout: cs.dialTimeout,
	})
	if err != nil {
		return nil, errutil.Explain(err, "create etcd client for %s failed", cs.endpoint)
	}
	log.Infof(ctx, starterTag, "create etcd client success")
	c.clients[key] = cli
	return cli, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from etcd, parses it according to the declared format, and installs a
// change watcher that triggers an application property refresh.
func (c *etcdCtrl) Load(ctx context.Context, optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		log.Error(ctx, starterTag, err,
			log.String("source", redactSource(source)),
			log.Msg("parse etcd source failed"))
		return nil, err
	}

	// The source's identity rides on the context from here on: every event this
	// load and its helpers print carries it without repeating it at each call
	// site.
	ctx = log.WithFields(ctx,
		log.String("endpoint", cs.endpoint),
		log.String("format", cs.format),
		log.String("key", cs.key))

	log.Debugf(ctx, starterTag, "loading etcd config")

	cli, err := c.clientFor(ctx, cs)
	if err != nil {
		log.Errorf(ctx, starterTag, err, "create etcd client failed")
		return nil, err
	}

	// Read first to learn the key's revision, then install the watcher seeded
	// with it; the watch then replays any change since that read instead of
	// skipping it. Registering regardless of the read's outcome keeps the
	// "missing optional key is still watched" behaviour.
	m, since, err := loadFromClient(ctx, cli, cs, optional)
	c.watch.registerWatcher(cli, cs, optional, since)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// loadFromClient reads the key once, applies the optional/empty rules, and
// parses it into flattened properties. It also returns the store revision the
// read observed (0 when unknown), which the caller seeds the watcher with so
// the watch resumes from exactly there.
func loadFromClient(ctx context.Context, cli etcdAPI, cs configSource, optional bool) (map[string]string, int64, error) {
	getCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(getCtx, cs.key)
	var since int64
	if resp != nil {
		since = resp.Header.Revision
	}
	if err != nil {
		if optional {
			log.Warn(ctx, starterTag,
				log.Err(err),
				log.Msg("skip optional config get key failed"))
			return nil, since, nil
		}
		log.Errorf(ctx, starterTag, err, "get etcd key failed")
		return nil, since, errutil.Explain(err, "get etcd key %s failed", cs.key)
	}
	if len(resp.Kvs) == 0 {
		if optional {
			log.Warnf(ctx, starterTag, "skip optional config key is empty")
			return nil, since, nil
		}
		err := errutil.Explain(nil, "etcd key %s is empty", cs.key)
		log.Errorf(ctx, starterTag, err, "etcd key is empty")
		return nil, since, err
	}

	content := resp.Kvs[0].Value
	m, err := reader.Read(cs.format, content)
	if err != nil {
		log.Error(ctx, starterTag, err, log.String("format", cs.format), log.Msg("parse etcd key failed"))
		return nil, since, errutil.Explain(err, "parse etcd key %s as %s failed", cs.key, cs.format)
	}

	log.Infof(ctx, starterTag, "load etcd config success")
	return flatten.Flatten(m), since, nil
}
