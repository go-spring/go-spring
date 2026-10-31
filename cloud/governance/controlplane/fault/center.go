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

// Package fault is the fault-injection CONTROL PLANE: one config ROOT, peer to
// governance, that owns the experiment document an operator publishes.
//
// It is deliberately NOT part of the policy root. Fault injection and
// policy (retry, breaker, rate limit, endpoint selection) are different kinds
// of thing:
//
//   - policy is stateless and replaced whole; an experiment has an identity and
//     cumulative guardrails that must survive a push;
//   - policy is tuned by the service owner; an experiment is armed by whoever
//     runs the drill, under different permissions and a different blast radius;
//   - a break in policy weakens protection; a break in an experiment injects
//     production failures.
//
// Sharing one document would force one writer (two platforms read-modify-writing
// it lose each other's changes), one validation (a bad experiment would reject
// an unrelated retry change), and one key space (config-center ACLs are granted
// by prefix, so "may tune retry" would imply "may arm a fire").
//
// The transport is shared, the document is not: this root consumes the same
// generic [source.Source] contract as every other root, but the transport itself
// (a file, a config center, a stream) is provided by whoever owns the
// deployment's configuration — a company-level source starter. The document is
// [kfault.Document] from the data plane: the root binds it and fans it out to
// the two direction managers. It lives under spring.fault.*. A process whose
// source starter provides no Source[kfault.Document] is simply not armed — a
// legitimate state that a single shared document could not express.
//
// The lifecycle is the document's presence: a rule appears and it starts; it
// disappears and its counters are released. Identity is (run id, rule name), so
// a reorder, an unrelated edit or a re-push inside one run never resets a live
// rule's counters, while a new run id starts fresh. There is no start/stop call
// and no persistence — injection is in-process only; a restart is a fresh
// process with fresh counters, and a future platform must treat it as such.
package fault

import (
	"context"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/controlplane/source"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/log"
)

// Center is the fault control plane: it holds this root's document and pushes
// each direction into its own manager.
//
// It owns the DOCUMENT, not the request path. The managers it drives are the
// same data-plane authorities the runtime builds its executors on; the center
// only calls their Refresh and exposes none of them. The request path is
// unaffected either way: Fire reads the manager's own atomic kernel.
//
// It is container-free: it knows sources only through the [source.Source]
// contract, so the gs wiring lives in starter.go.
//
// Refresh is not safe for concurrent use: the source serializes deliveries, and
// the initial load happens during wiring, before the source's loop starts.
type Center struct {
	// client and server are the two fault authorities this center arms. They are
	// injected, not constructed here, so the runtime and the center share the
	// same instances.
	client *clientfault.Manager
	server *serverfault.Manager

	// src is this root's source, bound for life. It is ALWAYS present: the
	// starter registers an empty [source.PushSource] when no company-level
	// source starter provides one, so a process with no fault transport arms
	// with an empty document (injects nothing) instead of leaving the field nil.
	// A deployment whose source starter DOES provide one fails loudly in that
	// adapter (unreadable file, unreachable console).
	src source.Source[kfault.Document]
}

// NewCenter arms the fault root: it applies the source's snapshot, then
// subscribes so every later push drives [Center.Refresh].
//
// Every constraint is a field expr tag judged at conf.Bind, so a source rejects
// a bad document before pushing; a misconfigured source fails startup in its own
// adapter (unreadable file, unreachable console). With no company-level source
// starter the package registers an empty PushSource, so this arms on an empty
// document and injects nothing.
//
// The initial document is applied during wiring, so a failure there fails
// startup instead of arming an unconfigured center. A later push has no caller
// to intervene; [Center.Refresh] has no failure mode today (see its doc).
//
// ctx is the wiring's own context — the starter hands it the app's root context
// via [gs.ContextProvider]; a hand-built caller passes whatever it has. The
// startup "config applied" log borrows it, the same way every later push's is.
func NewCenter(ctx context.Context, client *clientfault.Manager, server *serverfault.Manager,
	src source.Source[kfault.Document]) (*Center, error) {
	c := &Center{client: client, server: server, src: src}
	if err := c.Refresh(ctx, c.src.Snapshot()); err != nil {
		return nil, err
	}
	c.src.Subscribe(func(ctx context.Context, doc kfault.Document) {
		// A push has no caller to intervene.
		_ = c.Refresh(ctx, doc)
	})
	return c, nil
}

// Refresh adopts doc as this root's state and returns why it could not: it
// fills the drill id into both directions, then hands each to its manager,
// which commits it and rebuilds that direction's kernel. Wiring propagates the
// error and refuses to start; a source push has no caller to intervene.
//
// It has no failure mode today: every constraint is a field-level expr tag
// judged at conf.Bind, and the direction managers take a commit. The error
// return is the family contract, so both phases keep one shape and a future
// kernel check has somewhere to surface.
//
// It is the ONE entry point for both the initial snapshot and every push, whole
// document and idempotent: a source always delivers the complete document, never
// a delta, so "a push that changed nothing" is naturally a no-op. It never runs
// concurrently: the source serializes deliveries, and the initial load happens
// during wiring, before the source's loop starts.
func (c *Center) Refresh(ctx context.Context, doc kfault.Document) error {
	// The document's run id is the drill id of BOTH directions.
	if doc.RunID != "" {
		doc.Client.RunID = doc.RunID
		doc.Server.RunID = doc.RunID
	}

	c.client.Refresh(doc.Client)
	c.server.Refresh(doc.Server)
	log.Info(ctx, log.TagAppDef, "fault: config applied",
		log.String("run_id", doc.Client.RunID))
	return nil
}
