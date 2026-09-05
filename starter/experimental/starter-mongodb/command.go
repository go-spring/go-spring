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

// command.go is the "command seam" concept of this starter: the observe layer
// that instruments every MongoDB command via an event.CommandMonitor. The mongo
// driver v2 exposes no per-operation hook comparable to go-redis's ProcessHook,
// so observation rides the command monitor while resilience rides the dial seam
// (wrapped in client.go's Init) — the analog of starter-go-redis's command.go.
package StarterMongoDB

import (
	"context"
	"strconv"
	"sync"

	"go.mongodb.org/mongo-driver/v2/event"
)

// newCommandMonitor returns an event.CommandMonitor that drives this
// starter's own instrumentation (see observe.go) for every MongoDB command: a
// trace span, a duration/in-flight metric, and an access log. A command's observation is
// opened in Started and closed in Succeeded/Failed; events are correlated by
// (connection id, request id), which the driver guarantees is unique for an
// in-flight command.
//
// getObs lazily supplies the observer: newClient installs the monitor before
// Init builds the observer, so Init makes it available through the getter. Because no
// command runs before Init (the wrapper is not handed out until
// startup completes), the monitor never sees a nil observer in practice; the
// nil guard keeps the probe path (startup Ping) safe.
//
// Why hand-rolled against the v2 event API (not otelmongo): the official
// otelmongo instrumentation targets the v1 mongo driver and its CommandMonitor
// type is incompatible with the v2 driver this starter uses.
func newCommandMonitor(getObs func() *dbObserver) *event.CommandMonitor {
	var inFlight sync.Map // spanKey -> *dbSpan

	return &event.CommandMonitor{
		Started: func(ctx context.Context, e *event.CommandStartedEvent) {
			if obs := getObs(); obs != nil {
				_, sp := obs.Start(ctx, e.CommandName, e.DatabaseName)
				inFlight.Store(spanKey(e.ConnectionID, e.RequestID), sp)
			}
		},
		Succeeded: func(_ context.Context, e *event.CommandSucceededEvent) {
			if v, ok := inFlight.LoadAndDelete(spanKey(e.ConnectionID, e.RequestID)); ok {
				v.(*dbSpan).End(nil)
			}
		},
		Failed: func(_ context.Context, e *event.CommandFailedEvent) {
			if v, ok := inFlight.LoadAndDelete(spanKey(e.ConnectionID, e.RequestID)); ok {
				v.(*dbSpan).End(e.Failure)
			}
		},
	}
}

// spanKey uniquely identifies an in-flight command by its connection and
// request id, so the Succeeded/Failed event can find the span Started opened.
func spanKey(connID string, requestID int64) string {
	return connID + "/" + strconv.FormatInt(requestID, 10)
}
