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

// Package loadbalance is the ENDPOINT-SELECTION control-plane root: the strategy,
// its parameters, the outlier-suspension thresholds and the metadata selector.
//
// Selection is resolved per CONNECTION at wiring time, not per call, so its
// document has no Action/Match level: one rule per service label.
//
// The document is [kloadbalance.Settings] from the data plane — the root binds
// it, validates it at the source boundary and pushes it whole.
package loadbalance

import (
	"context"

	kloadbalance "go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/log"
)

// Center is the selection control plane: it holds this root's document and
// pushes it into the endpoint-selection authority.
//
// It owns the DOCUMENT, not the request path. The authority it drives is the
// same data-plane object the runtime holds; the center only calls
// [kloadbalance.Manager.Apply] and exposes nothing else. The manager rebuilds the
// affected chains eagerly, so a push takes effect at the next Pick.
//
// It is container-free: it knows sources only through [source.Source], so the gs
// wiring lives in starter.go.
//
// Refresh is not safe for concurrent use: the source serializes deliveries, and
// the initial load happens during wiring, before the source's loop starts.
type Center struct {
	mgr *kloadbalance.Manager
	src source.Source[kloadbalance.Settings]
}

// NewCenter arms the selection root: it applies the source's snapshot, then
// subscribes so every later push drives [Center.Refresh].
//
// The initial document is applied during wiring, so a document that cannot
// serve fails startup instead of arming an unconfigured center. Later pushes
// have no caller to intervene: [Center.Refresh] logs the cause and keeps the
// last good document serving.
func NewCenter(ctx context.Context, mgr *kloadbalance.Manager, src source.Source[kloadbalance.Settings]) (*Center, error) {
	c := &Center{mgr: mgr, src: src}
	if err := c.Refresh(ctx, c.src.Snapshot()); err != nil {
		return nil, err
	}
	c.src.Subscribe(func(ctx context.Context, doc kloadbalance.Settings) {
		// A push has no caller; Refresh logs any rejection.
		_ = c.Refresh(ctx, doc)
	})
	return c, nil
}

// Refresh adopts doc as this root's state and returns the cause when it could
// not, logging each rejection at its branch. The caller decides what a failure
// means: wiring propagates it and refuses to start; a source push ignores it —
// the log is the intervention.
func (c *Center) Refresh(ctx context.Context, doc kloadbalance.Settings) error {
	if err := doc.Validate(); err != nil {
		log.Error(ctx, log.TagAppDef, err, "loadbalance: invalid config")
		return err
	}
	c.mgr.Apply(ctx, doc)
	log.Info(ctx, log.TagAppDef, "loadbalance: config applied")
	return nil
}
