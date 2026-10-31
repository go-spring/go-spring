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

// Package admission is the INBOUND-ADMISSION control-plane root: rate limit,
// concurrency cap and handling budget for the entrances this process serves.
//
// It is the server-side counterpart of the outbound roots: one document, one
// source, one last-good, keyed by the entrance label a server adapter passes.
// The document is [admission.Settings] from the data plane — the root binds it
// (conf.Bind validates at the source boundary) and pushes it whole.
package admission

import (
	"context"

	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	"go-spring.org/log"
)

// Center is the admission control plane: it holds this root's document and
// pushes it into the inbound admission authority.
//
// It owns the DOCUMENT, not the request path. The authority it drives is the
// same data-plane object the runtime holds; the center only calls
// [admission.Manager.Apply] and exposes nothing else. The manager hands each
// registered Guard its resolved policy; a Guard follows it on its next Admit.
//
// It is container-free: it knows sources only through [source.Source], so the gs
// wiring lives in starter.go.
//
// Refresh is not safe for concurrent use: the source serializes deliveries, and
// the initial load happens during wiring, before the source's loop starts.
type Center struct {
	mgr *admission.Manager
	src source.Source[admission.Settings]
}

// NewCenter arms the admission root: it applies the source's snapshot, then
// subscribes so every later push drives [Center.Refresh].
//
// The initial document is applied during wiring, so a document that cannot
// serve fails startup instead of arming an unconfigured center. Later pushes
// have no caller to intervene: [Center.Refresh] logs the cause and keeps the
// last good document serving.
func NewCenter(ctx context.Context, mgr *admission.Manager, src source.Source[admission.Settings]) (*Center, error) {
	c := &Center{mgr: mgr, src: src}
	if err := c.Refresh(ctx, c.src.Snapshot()); err != nil {
		return nil, err
	}
	c.src.Subscribe(func(ctx context.Context, doc admission.Settings) {
		// A push has no caller; Refresh logs any rejection.
		_ = c.Refresh(ctx, doc)
	})
	return c, nil
}

// Refresh adopts doc as this root's state and returns the cause when it could
// not, logging each rejection at its branch. The caller decides what a failure
// means: wiring propagates it and refuses to start; a source push ignores it —
// the log is the intervention.
func (c *Center) Refresh(ctx context.Context, doc admission.Settings) error {
	if err := doc.Validate(); err != nil {
		log.Error(ctx, log.TagAppDef, err, "admission: invalid config")
		return err
	}
	if err := c.mgr.Apply(doc); err != nil {
		log.Error(ctx, log.TagAppDef, err, "admission: config apply failed")
		return err
	}
	log.Info(ctx, log.TagAppDef, "admission: config applied")
	return nil
}
