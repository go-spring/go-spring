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

package governance

import (
	"context"
	"sync"
)

// Source is governance's own awareness contract: a provider of the governance
// [Config] snapshot plus change subscription. The [Center] depends only on this
// contract — never on gs.Dync or any config mechanism — so any pusher (a
// governance console stream, a dedicated config-center listener, a static
// injection) can drive the whole center, resilience AND fault alike, by
// implementing two methods. This mirrors how Sentinel-Golang's ext/datasource
// keeps the rule core ignorant of where bytes come from: the transport is an
// adapter concern, the rule model and the fan-out are the center's.
//
// Deliberate omissions from the contract:
//   - No error returns. The center cannot roll back or retry a bad push — how
//     to treat one (log, metric, keep the last good snapshot) is the source's
//     concern; the contract is "everything you push, you vouch for".
//   - No Close in the interface. Lifecycle belongs to the implementation;
//     [Center.Destroy] closes sources that happen to implement
//     interface{ Close() error } via a type assertion, so implementers with no
//     resources don't have to write an empty method.
//   - A single callback. The center is the only consumer, which keeps the
//     implementation surface at minimum.
type Source interface {
	// Snapshot returns the current config. It must be safe for concurrent use
	// and return the latest committed value; before anything is pushed it
	// returns a zero Config (a disabled center).
	Snapshot() Config

	// Subscribe registers cb, invoked with each new config after it commits,
	// and with the context of the change that produced it. At most one
	// subscription is consumed — the center is the only consumer — so a second
	// Subscribe may replace the first. cb must be safe for concurrent
	// invocation; delivering synchronously inside Subscribe is allowed (the
	// center's guard tolerates it).
	//
	// The context is the change's own: it carries whatever the source named the
	// change with (its coordinates, the change's trace_id), so the consumer logs
	// what it did about the change against the same identity. A source that has
	// nothing to say about a delivery passes [context.Background].
	Subscribe(cb func(ctx context.Context, cfg Config))
}

// PushSource is a ready-made mutable [Source]: it holds one snapshot and
// forwards pushes to its subscriber. It is the shared core of every push-style
// adapter — the file, http, etcd and nacos sources in the governance starter
// family each wrap one behind their own transport — and the building block for
// console-stream and static-injection integrations: create one, hand it to the
// center with [Center.SetSource], then Push on every upstream event:
//
//	src := governance.NewPushSource(governance.Config{})
//	ctr.SetSource(src)
//	// later, on each console event:
//	src.Push(cfg)
//
// The usual route is the bean one instead: export a [Source] and the container
// hands it straight to the center's constructor; SetSource is the eager path for
// a caller that holds the center and wants to replace whatever is bound.
//
// Safe for concurrent use.
type PushSource struct {
	mu  sync.Mutex
	cfg Config
	cb  func(ctx context.Context, cfg Config)
}

// NewPushSource returns a PushSource holding cfg as its initial snapshot.
func NewPushSource(cfg Config) *PushSource {
	return &PushSource{cfg: cfg}
}

// Snapshot returns the latest pushed config.
func (p *PushSource) Snapshot() Config {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cfg
}

// Subscribe registers cb. If a callback was already registered it is replaced
// (the single-consumer contract makes replacement, not fan-out, the right
// semantic).
func (p *PushSource) Subscribe(cb func(ctx context.Context, cfg Config)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cb = cb
}

// Push adopts cfg as the new snapshot and, once subscribed, delivers it to the
// callback together with ctx, the change's own context. Before any Subscribe the
// push only moves the snapshot: the center picks it up when it binds (its bind
// adopts Snapshot()).
func (p *PushSource) Push(ctx context.Context, cfg Config) {
	p.mu.Lock()
	p.cfg = cfg
	cb := p.cb
	p.mu.Unlock()
	if cb != nil {
		cb(ctx, cfg)
	}
}
