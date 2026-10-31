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

// Package StarterGovernanceEtcd adapts etcd as a governance rule source: it
// watches one etcd key holding the selection rule document and pushes every
// changed version into the selection center through the
// [go-spring.org/cloud/governance/controlplane/source.Source[loadbalance.Settings]] contract.
//
// It is the etcd member of the selection-source adapter family — the
// Sentinel-datasource shape. The rule document is governance's own, not an
// application-config import, so enabling governance never turns on app-property
// refresh and vice versa; this module is therefore separate from
// starter-config-etcd even though both speak to etcd. The source owns its
// client exclusively, so it is free to carry its own instrumentation
// independently of the config-import bootstrap client.
//
// Configure with ${source.etcd.*}:
//
//	source.etcd.endpoints=127.0.0.1:2379
//	source.etcd.key=/app/governance.yaml
//
// The bean is registered only when that key is present, so importing this
// package is inert otherwise.
package StarterGovernanceEtcd

import (
	"context"
	"reflect"
	"time"

	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/randutil"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var starterTag = log.RegisterAppTag("governance", "etcd")

func init() {
	gs.Module(gs.OnProperty("source.etcd"), func(r gs.BeanProvider, _ flatten.Storage) error {
		r.Provide(func(ctx *gs.ContextProvider, c governanceEtcdConfig) (*EtcdSource, error) {
			return NewEtcdSource(ctx.Context, c)
		}, gs.IndexArg(1, gs.TagArg("${source.etcd:=}"))).
			Init((*EtcdSource).Init).
			Destroy((*EtcdSource).Close).
			Export(gs.As[source.Source[loadbalance.Settings]]()).Caller(1)
		return nil
	})
}

// etcdKV is the subset of *clientv3.Client the source needs — an interface so
// tests can drive the push chain without a live etcd.
type etcdKV interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
	Close() error
}

// EtcdSource is a source.Source[loadbalance.Settings] backed by one etcd key. The initial Get
// seeds the snapshot (a missing key or bad document fails construction, so a
// misconfigured path surfaces at startup); the Watch stream then delivers each
// PUT, which re-parses through source.Parse and pushes when the rules
// actually changed. A bad value keeps the last good snapshot and logs.
type EtcdSource struct {
	kv     etcdKV
	key    string
	format string
	doc    string // latest document bytes (for dedupe before parsing)

	// source.PushSource[loadbalance.Settings] is embedded so this transport inherits the [Source]
	// read and subscribe pair (the watch stream feeds it through apply).
	// Embedding also promotes Push — the INTERNAL commit path that skips apply's
	// parse and change-dedup guards — so callers must never call Push directly.
	*source.PushSource[loadbalance.Settings]

	// cancel stops the watch stream. It is set by Init and called by Close with
	// no lock: gs runs Init synchronously while wiring the bean and Destroy
	// exactly once at shutdown, so Close runs at most once and only in that order
	// — the nil check tolerates a source dropped without ever being initialized.
	cancel context.CancelFunc
}

// NewEtcdSource builds the client from the bound config and seeds the snapshot
// from the key's current value.
func NewEtcdSource(ctx context.Context, c governanceEtcdConfig) (*EtcdSource, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Endpoints,
		Username:    c.Username,
		Password:    c.Password,
		DialTimeout: c.DialTimeout,
	})
	if err != nil {
		return nil, errutil.Explain(err, "governance etcd source: create client for %v failed", c.Endpoints)
	}
	return newEtcdSource(ctx, cli, c.Key, sourceFormat(c.Format, c.Key))
}

// newEtcdSource seeds the snapshot from the key's current value on an existing
// client. The Get is bounded by a timeout on top of the caller's ctx (the
// injected app ctx).
func newEtcdSource(seedCtx context.Context, kv etcdKV, key, format string) (*EtcdSource, error) {
	ctx, cancel := context.WithTimeout(seedCtx, 5*time.Second)
	defer cancel()
	resp, err := kv.Get(ctx, key)
	if err != nil {
		return nil, errutil.Explain(err, "governance etcd source: get %s failed", key)
	}
	if len(resp.Kvs) == 0 {
		return nil, errutil.Explain(nil, "governance etcd source: key %s is empty", key)
	}
	doc := string(resp.Kvs[0].Value)
	cfg, err := source.Parse[loadbalance.Settings](key, []byte(doc), format)
	if err != nil {
		return nil, err
	}
	return &EtcdSource{kv: kv, key: key, format: format, doc: doc, PushSource: source.NewPushSource(cfg)}, nil
}

// Init opens the watch stream (the gs bean lifecycle hook). The stream runs
// until Close; watch responses without events (compactions, auth refreshes)
// are ignored. The stream ctx carries the key field once, so every delivery
// apply handles logs with it.
func (s *EtcdSource) Init() error {
	ctx, cancel := context.WithCancel(log.RootFields(
		log.String("source", "etcd"),
		log.String("key", s.key),
	))
	s.cancel = cancel

	ch := s.kv.Watch(ctx, s.key)
	go func() {
		for wr := range ch {
			for _, ev := range wr.Events {
				if ev.Type == clientv3.EventTypePut {
					if data := string(ev.Kv.Value); data != s.doc {
						s.apply(ctx, data)
					}
				}
			}
		}
	}()
	return nil
}

// Close stops the watch stream and closes the source's own client — the client
// is built by the bean ctor and dies with the bean. Implements the
// optional-close contract the selection center probes for on Destroy.
func (s *EtcdSource) Close() error {
	if s.cancel != nil { // never initialized — only the client to close
		s.cancel()
	}
	return s.kv.Close()
}

// apply parses one delivered value and, when the rules actually changed,
// pushes it. The caller drops byte-equal re-deliveries; bad values push
// nothing. The stream ctx already carries the key field; the delivery only
// mints its trace_id on top — one delivery is one path, and it continues into
// the center that applies it, so the trace_id is the one both lines carry.
func (s *EtcdSource) apply(ctx context.Context, data string) {
	ctx = log.WithFields(ctx, log.String("trace_id", randutil.Hex(16)))
	cfg, err := source.Parse[loadbalance.Settings](s.key, []byte(data), s.format)
	if err != nil {
		log.Error(ctx, starterTag, err, "governance etcd source got an invalid value (keeping last good config)")
		return
	}
	s.doc = data
	if reflect.DeepEqual(s.Snapshot(), cfg) {
		return
	}
	s.Push(ctx, cfg)
}
