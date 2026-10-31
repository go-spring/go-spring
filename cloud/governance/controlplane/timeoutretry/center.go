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

// Package timeoutretry is the TIMEOUT + RETRY control-plane root: the call
// deadline, the retry schedule and the label-scoped retry budget.
//
// The scope is written in the rule's Action selector: "-" means the whole
// service, a real exact/prefix means one interface. A real interface rule always
// wins over the service marker. There is no global level: a label with no
// matching rule runs unbounded.
//
// The document is [resilience.LimitsSettings] from the data plane — the root
// binds it, validates it at the source boundary and pushes it whole.
package timeoutretry

import (
	"context"

	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/log"
)

// Center is the limits control plane: it holds this root's document and pushes
// it into the outbound resilience authority.
//
// It owns the DOCUMENT, not the request path. The authority it drives is the
// same data-plane object the runtime holds; the center only calls
// [resilience.Manager.ApplyLimits] and exposes nothing else. The request path is
// unaffected either way: an executor reads the manager's published snapshot on
// its next call.
//
// It is container-free: it knows sources only through [source.Source], so the gs
// wiring lives in starter.go.
//
// Refresh is not safe for concurrent use: the source serializes deliveries, and
// the initial load happens during wiring, before the source's loop starts.
type Center struct {
	mgr *resilience.Manager
	src source.Source[resilience.LimitsSettings]
}

// NewCenter arms the limits root: it applies the source's snapshot, then
// subscribes so every later push drives [Center.Refresh].
//
// The initial document is applied during wiring, so a document that cannot
// serve fails startup instead of arming an unconfigured center. Later pushes
// have no caller to intervene: [Center.Refresh] logs the cause and keeps the
// last good document serving.
func NewCenter(ctx context.Context, mgr *resilience.Manager, src source.Source[resilience.LimitsSettings]) (*Center, error) {
	c := &Center{mgr: mgr, src: src}
	if err := c.Refresh(ctx, c.src.Snapshot()); err != nil {
		return nil, err
	}
	c.src.Subscribe(func(ctx context.Context, doc resilience.LimitsSettings) {
		// A push has no caller; Refresh logs any rejection.
		_ = c.Refresh(ctx, doc)
	})
	return c, nil
}

// Refresh adopts doc as this root's state and returns the cause when it could
// not, logging each rejection at its branch. The caller decides what a failure
// means: wiring propagates it and refuses to start; a source push ignores it —
// the log is the intervention.
func (c *Center) Refresh(ctx context.Context, doc resilience.LimitsSettings) error {
	if err := doc.Validate(); err != nil {
		log.Error(ctx, log.TagAppDef, err, "timeoutretry: invalid config")
		return err
	}
	if err := c.mgr.ApplyLimits(doc); err != nil {
		log.Error(ctx, log.TagAppDef, err, "timeoutretry: config apply failed")
		return err
	}
	log.Info(ctx, log.TagAppDef, "timeoutretry: config applied")
	return nil
}
