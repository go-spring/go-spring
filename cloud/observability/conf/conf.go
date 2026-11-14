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

// Package conf is the shared instrumentation funnel for application-wide
// property refreshes. Every config backend (nacos, etcd, consul, vault, k8s,
// file, ...) fires the same refresh when its watch reports a change; this
// package records that one fleet-wide event (metrics + one log line) without
// importing the governance stack, so config starters stay free of
// chain/guard/breaker.
package conf

import (
	"context"
	"time"

	"go-spring.org/log"
)

var configTag = log.RegisterAppTag("config", "")

// Status attribute values. The statuses are exclusive, so
// config.refresh.total summed over status is the number of refreshes
// triggered — there is no separate counter that could double-count.
const (
	statusOK    = "ok"
	statusError = "error"
)

// Refresh is the shared funnel for property-refresh triggers. Every config
// backend (nacos, etcd, consul, vault, k8s, file, ...) fires the same
// application-wide refresh when its watch reports a change; without a shared
// funnel, "was the fleet actually refreshed, and did it fail" is answerable
// only by grepping per-module logs.
//
// It wraps the refresh function instead of referencing it: callers pass their
// own (typically gs.RefreshProperties), so this package stays spring-free.
//
//	conf.Refresh(ctx, func(ctx context.Context) error {
//		return gs.RefreshProperties(ctx)
//	})
//
// Logs are recorded here too, centrally: a refresh is a rare, fleet-wide
// event, so the funnel is also the natural place for its log line. Callers
// log only their backend events (key deleted, watch retrying, poll failed) —
// the refresh outcome itself is this function's to report.
//
// Refresh executes fn once and records the status: one exclusive value on
// config.refresh.total, the elapsed time on config.refresh.duration, and the
// wall-clock time on config.refresh.last_success_timestamp when fn succeeded
// (refresh stalls show up as that gauge going flat; alert on
// time() - last_success_timestamp exceeding a threshold). fn's error is
// returned unchanged — this is instrumentation, not error policy.
//
// ctx is the trigger's context and the only carrier of identity: whatever
// fields it was given at the head of the path (the backend's coordinates, the
// path's trace_id) are what the round's records and logs show. The funnel adds
// none of its own, so fn must log with the context it is handed, not one it
// closed over. The refresh itself runs on the application's own context (see
// gs.RefreshProperties), which re-roots these fields onto it.
func Refresh(ctx context.Context, fn func(ctx context.Context) error) error {
	return RefreshWith(ctx, fn, defaultObserver())
}

// RefreshWith is the injection point for a custom observability adapter. A nil
// observer disables observation; the log line is still written.
func RefreshWith(ctx context.Context, fn func(ctx context.Context) error, obs Observer) error {
	start := time.Now()
	err := fn(ctx)
	elapsed := time.Since(start)

	status := statusOK
	if err != nil {
		status = statusError
	}
	if obs != nil {
		obs.Refresh(ctx, status, elapsed)
	}

	// One status value drives both the metrics and the log, so they can never
	// disagree. The log level carries the outcome: a failed refresh at Warn
	// (the previous snapshot is retained, so it is degraded, not broken), a
	// successful one at Info (refreshes are rare and always worth a line).
	summary := []log.Field{
		log.String("status", status),
		log.Float("duration_ms", float64(elapsed.Nanoseconds())/1e6),
	}
	if err != nil {
		log.Warn(ctx, configTag, "property refresh failed", append(summary, log.Err(err))...)
	} else {
		log.Info(ctx, configTag, "refresh properties success", summary...)
	}
	return err
}
