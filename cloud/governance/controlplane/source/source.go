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

// Package source is the control plane's TRANSPORT: it knows how a document
// arrives, never what the document means.
//
// A config ROOT is a document that is validated, applied and rejected as a
// whole; each root owns its own key and its own last-good snapshot. The roots
// (policy, fault, ...) are peers and must not import each other, so the
// transport they share cannot live in any of them. It lives here, generic over
// the document type:
//
//	Source[T]      snapshot + change subscription (what a root's center consumes)
//	PushSource[T]  the mutable core every adapter embeds
//	FileSource[T]  watch one file (fsnotify, atomic-rename tolerant)
//	FileConfig     the file adapter's bindable key (a root binds it under its
//	                own prefix)
//	Parse[T]       flatten + conf.Bind; T declares its key space on its
//	                top-level value tags, so Parse takes no prefix
//
// Remote transports (nacos, etcd, a console) are their own starter modules and
// implement [Source] on top of [PushSource]; the core package owns no remote
// client of its own.
//
// Why generic rather than an `any` document: [Parse] is the only type-dependent
// step, and it is contained by T itself — T's top-level value tags declare its
// keys. Every transport is therefore implemented ONCE and instantiated per root
// over that root's settings/document type; a new root costs one such type and
// its own (explicit) gs wiring, never a second copy of the fsnotify loop.
//
// This package deliberately provides NO shared "register" helper. Each root's
// starter.go writes its own gs.Module/gs.Provide so the wiring stays readable:
// which key arms which bean, what it exports, and what its lifecycle is are all
// visible at the root, not hidden behind a generic function.
//
// The boundary this package must never cross: it must not import the packages
// that define documents (policy, fault, ...). If a document type ever
// appears in an import here, the transport has been welded to a root again.
//
// One root, one key. Each root's document is addressed independently (its own
// file or dataId), which is what lets a deployment feed the two roots from
// different platforms without either one being able to clobber the other. Do
// not share one byte stream between two roots: with no root-key guard, an
// emptied section is indistinguishable from a deliberate disarm, so the
// document a source owns must be the document it publishes.
//
// A root configures AT MOST one source: two sources would each believe they
// hold the truth, and with no root-key guard neither could tell a deliberate
// disarm from a clobbered document.
package source

import (
	"context"

	"go-spring.org/log"
)

// sourceTag identifies transport-level log lines (reload failures). It
// names the TRANSPORT, not a root: the document's coordinates (path/URL) are
// log fields, and the root is named by the center that applies the document.
var sourceTag = log.RegisterAppTag("controlplane", "")

// Source is one config root's awareness contract: a provider of that root's
// document snapshot plus a change subscription. The root's center depends only
// on this contract — never on gs.Dync or any config mechanism — so any pusher
// (a console stream, a dedicated config-center listener, a static injection)
// drives it by implementing two methods. T is the root's document type: each
// root instantiates it over its own type (the policy roots' data-plane
// Settings types, fault's kernel document), which are distinct types, so
// injection never confuses them.
//
// Deliberate omissions from the contract:
//   - No error returns. The center cannot roll back or retry a bad push — how
//     to treat one (log, metric, keep the last good snapshot) is the source's
//     concern; the contract is "everything you push, you vouch for".
//   - No Close in the interface. Lifecycle belongs to the implementation; a
//     source that owns resources exposes Close() and the root's starter
//     registers it as the bean's Destroy hook, so implementers with no resources
//     don't have to write an empty method.
//   - A single callback. The root's center is the only consumer, which keeps
//     the implementation surface at minimum.
//
// The FIRST READ is part of construction, and this is the one thing an
// implementer must get right. A center arms itself synchronously from
// Snapshot() when it starts, so by the time the source's constructor RETURNS,
// Snapshot() must already hold the latest committed document — a remote source
// performs its first fetch inside its constructor (bounded by a timeout) and
// fails construction when that fetch fails. That is what makes a misconfigured
// source — a wrong path, an unreachable console — fail startup instead of
// leaving the center silently on its empty document: a zero T returned because
// nothing was loaded is a bug, not a state.
//
// This is why the first read is NOT deferred to a goroutine or a lazy accessor.
// A source that started loading asynchronously would hand the center a zero T
// and the process would come up unconfigured until the first push — the
// fail-loud guarantee traded for a silent gap. A zero T is legal ONLY as "an
// empty document that configures nothing" that the source actually read; it
// never means "not loaded yet".
type Source[T any] interface {
	// Snapshot returns the latest committed value. It is a pure in-memory
	// read: it must NOT fetch anything, because a center reads it while arming
	// and a blocking read there would stall startup. It carries no concurrency
	// requirement of its own: the one cross-goroutine read (a center's initial
	// load, during wiring) happens before the source's delivery goroutine
	// starts. A source always has a committed value by the time its constructor
	// returns (see the type doc); a zero T means the loaded document configures
	// nothing.
	Snapshot() T

	// Subscribe registers cb, invoked with each new config after it commits, and
	// with the context of the change that produced it. At most one subscription
	// is consumed — the root's center is the only consumer — so a second
	// Subscribe may replace the first. Deliveries are SERIALIZED: a source
	// delivers from one goroutine at a time, so cb never runs concurrently with
	// itself — a multi-goroutine pusher funnels its events into one delivery
	// goroutine before calling cb. This is what lets a center apply documents
	// without its own lock; a source that violates it is buggy, not defended
	// against. Delivering synchronously inside Subscribe is allowed.
	//
	// The context is the change's own: it carries whatever the source named the
	// change with (its coordinates, the change's trace_id), so the consumer logs
	// what it did about the change against the same identity. A source that has
	// nothing to say about a delivery passes [context.Background].
	//
	// cb has no error return because it never sees a bad document: validation is
	// part of committing, not of consuming. Adapters push only what [Parse]
	// accepted — bind plus the document type's own Validate() — and keep the last
	// good snapshot on any failure, so cb's job is applying a change, not judging
	// one. A failure inside cb goes to logs/metrics, never back up the channel.
	Subscribe(cb func(ctx context.Context, cfg T))
}

// PushSource is a ready-made mutable [Source]: it holds one snapshot and
// forwards pushes to its subscriber. It is the shared core of every push-style
// adapter — the file source embeds one behind its own transport — and
// the building block for console-stream and static-injection integrations:
// create one, hand it to a center, then Push on every upstream event:
//
//	src := source.NewPushSource(resilience.BreakerSettings{})
//	// later, on each console event:
//	src.Push(ctx, cfg)
//
// PushSource carries NO synchronization at all — the single-goroutine
// discipline of [Source] covers it: Push and Snapshot run on the delivery
// goroutine, and the one cross-goroutine read (a center's initial load during
// wiring) happens before that goroutine exists. A caller that violates the
// discipline is buggy, not defended against.
type PushSource[T any] struct {
	// cfg is the committed snapshot.
	cfg T
	// cb is the registered callback: set once during wiring, when the center
	// subscribes (always before any delivery), and never mutated afterwards — see
	// the serialized-deliveries contract.
	cb func(ctx context.Context, cfg T)
}

// NewPushSource returns a PushSource holding cfg as its initial snapshot. cfg is
// a COMMITTED document, not a placeholder: a caller that passes a zero T is
// saying "start on an empty document", not "I have nothing yet" — the center
// adopts it while arming exactly as it adopts any other snapshot (see
// [Source]). Adapters load the real document in their own constructor and only
// then install it (directly, as a PushSource value), so what the center reads on
// bind is always what the source actually vouches for.
func NewPushSource[T any](cfg T) *PushSource[T] {
	return &PushSource[T]{cfg: cfg}
}

// Snapshot returns the latest pushed config.
func (s *PushSource[T]) Snapshot() T {
	return s.cfg
}

// Subscribe registers cb. If a callback was already registered it is replaced
// (the single-consumer contract makes replacement, not fan-out, the right
// semantic). It must complete before any concurrent Push — see the cb field.
func (s *PushSource[T]) Subscribe(cb func(ctx context.Context, cfg T)) {
	s.cb = cb
}

// Push adopts cfg as the new snapshot and, once subscribed, delivers it to the
// callback together with ctx, the change's own context. Before any Subscribe the
// push only moves the snapshot: the center picks it up when it binds (its bind
// adopts Snapshot()).
func (s *PushSource[T]) Push(ctx context.Context, cfg T) {
	s.cfg = cfg
	if s.cb != nil {
		s.cb(ctx, cfg)
	}
}
