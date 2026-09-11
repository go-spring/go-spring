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

package StarterConfigEtcd

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/starter-governance/rules"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// This file adds a DIRECT etcd watcher for governance rules — the
// Sentinel-datasource shape: governance holds a dedicated key (its own config
// document, not a shared app-config import), and rule pushes refresh
// governance only, never the whole application's properties. The source owns
// its client exclusively (built here, closed on Destroy) — fully isolated
// from the config-import bootstrap client, and free to carry its own
// instrumentation. Document parsing goes through governance.ParseRules, so
// the key's value is byte-compatible with a starter-governance file source's
// rules file.
//
// Configure with (govern.source.etcd.*):
//
//	govern.source.etcd.endpoint=127.0.0.1:2379
//	govern.source.etcd.key=/app/govern.yaml

// governEtcdConfig binds ${govern.source.etcd.*}.
type governEtcdConfig struct {
	Endpoint string `value:"${endpoint}" expr:"$ != ''"`
	Key      string `value:"${key}" expr:"$ != ''"`
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`
	// Format overrides document-format detection; by default it is inferred
	// from the key's extension, else properties.
	Format string `value:"${format:=}"`
}

func init() {
	gs.Module(gs.OnProperty("govern.source.etcd"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c governEtcdConfig
		if err := conf.Bind(p, &c, "${govern.source.etcd:=}"); err != nil {
			return err
		}

		cs := configSource{
			endpoint: c.Endpoint,
			key:      c.Key,
			username: c.Username,
			password: c.Password,
			format:   c.Format,
		}
		if cs.format == "" {
			if ext := strings.TrimPrefix(extOf(c.Key), "."); ext != "" {
				cs.format = ext
			} else {
				cs.format = "properties"
			}
		}
		cs.dialTimeout = 5 * time.Second

		r.Provide(func() (*EtcdSource, error) {
			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   []string{cs.endpoint},
				Username:    cs.username,
				Password:    cs.password,
				DialTimeout: cs.dialTimeout,
			})
			if err != nil {
				return nil, errutil.Explain(err, "create etcd client for %s failed", cs.endpoint)
			}
			return NewEtcdSource(cli, cs.key, cs.format)
		}).
			Init((*EtcdSource).Init).Destroy((*EtcdSource).Close).
			Export(gs.As[governance.Source]()).Caller(1)
		return nil
	})
}

// extOf returns the dotted extension of name ("" when none).
func extOf(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i:]
	}
	return ""
}

// etcdKV is the slice of *clientv3.Client the source needs — an interface so
// tests can drive the push chain without a live etcd.
type etcdKV interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
	Close() error
}

// EtcdSource is a governance.Source backed by one etcd key. The initial Get
// seeds the snapshot (a missing key or bad document fails construction, so a
// misconfigured path surfaces at startup); the Watch stream then delivers each
// PUT, which re-parses through governance.ParseRules and pushes when the rules
// actually changed. A bad value keeps the last good snapshot and logs.
type EtcdSource struct {
	kv     etcdKV
	key    string
	format string
	doc    string // latest document bytes (for dedupe before parsing)

	mu     sync.Mutex
	cfg    governance.Config
	cb     func(governance.Config)
	cancel context.CancelFunc
}

// NewEtcdSource seeds the snapshot from the key's current value.
func NewEtcdSource(kv etcdKV, key, format string) (*EtcdSource, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := kv.Get(ctx, key)
	if err != nil {
		return nil, errutil.Explain(err, "governance etcd source: get %s failed", key)
	}
	if len(resp.Kvs) == 0 {
		return nil, errutil.Explain(nil, "governance etcd source: key %s is empty", key)
	}
	doc := string(resp.Kvs[0].Value)
	cfg, err := rules.Parse(key, []byte(doc), format)
	if err != nil {
		return nil, err
	}
	return &EtcdSource{kv: kv, key: key, format: format, doc: doc, cfg: cfg}, nil
}

// Init opens the watch stream (the gs bean lifecycle hook). The stream runs
// until Close; watch responses without events (compactions, auth refreshes)
// are ignored.
func (s *EtcdSource) Init() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	ch := s.kv.Watch(ctx, s.key)
	go func() {
		for wr := range ch {
			for _, ev := range wr.Events {
				if ev.Type == clientv3.EventTypePut {
					s.apply(string(ev.Kv.Value))
				}
			}
		}
	}()
	return nil
}

// Close stops the watch stream and closes the source's own client — the
// client is built by the bean ctor and dies with the bean, fully isolated
// from the config-import bootstrap client. Implements the
// optional-close contract the governance center probes for on Destroy.
func (s *EtcdSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	return s.kv.Close()
}

// Snapshot returns the latest good snapshot.
func (s *EtcdSource) Snapshot() governance.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Subscribe registers cb as the push target (the center is the only consumer).
func (s *EtcdSource) Subscribe(cb func(governance.Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cb = cb
}

// apply parses one delivered value and, when the rules actually changed,
// swaps the snapshot and pushes. Byte-equal re-deliveries and bad values push
// nothing.
func (s *EtcdSource) apply(data string) {
	if data == s.doc {
		return
	}
	cfg, err := rules.Parse(s.key, []byte(data), s.format)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "governance etcd source: key %s got an invalid value (keeping last good config): %v", s.key, err)
		return
	}

	s.mu.Lock()
	unchanged := reflect.DeepEqual(s.cfg, cfg)
	s.cfg, s.doc = cfg, data
	cb := s.cb
	s.mu.Unlock()

	if unchanged {
		return
	}
	if cb != nil {
		cb(cfg)
	}
}
