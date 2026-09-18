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

// observe.go is the module-local observability for Redis commands: the access
// log plus the DB family's operation instruments. Spans come from redisotel
// (installed by instrument() in starter.go), so this hook opens none — one
// emitted here would duplicate redisotel's.
//
// The operation metrics are NOT redisotel's. redisotel emits the connection
// POOL metrics (db.client.connections.*), which say nothing about how long a
// command took or how many are in flight; without these two, this backend
// would be the one DB member whose latency is invisible in the family's
// vocabulary.
package StarterGoRedis

import (
	"context"
	"fmt"
	"go-spring.org/stdlib/strutil"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// redisSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const redisSystem = "redis"

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set, shared with the other DB backends.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// accessTag is the static access-log tag for Redis commands, registered once
// at package init so logger config can address it (_app_redis_access).
var accessTag = log.RegisterAppTag("redis", "access")

// skipOps are the commands the access log suppresses entirely. Local decision,
// not config: the health-check PING (health/health.go) fires periodically per
// instance and would flood the log with uninteresting success lines.
var skipOps = map[string]struct{}{"ping": {}}

// newObserveHook builds the family's operation instruments from whatever meter
// provider is current — at attach time, not package init, so an SDK installed
// later than this package's init still receives the records.
func newObserveHook() *observeHook {
	m := otel.Meter("go-spring.org/starter-go-redis")
	duration, _ := m.Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of redis client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	active, _ := m.Int64UpDownCounter("db.client.active_requests",
		metric.WithDescription("Number of in-flight redis client operations"),
		metric.WithUnit("{request}"))
	return &observeHook{duration: duration, active: active}
}

// inflightOf names the in-flight gauge's dimensions. The +1 taken when a
// command starts and the -1 taken when it ends must carry identical
// attributes, or the gauge never balances — so both go through here.
func inflightOf(op string) metric.MeasurementOption {
	return metric.WithAttributes(
		attribute.String("db.system", redisSystem),
		attribute.String("db.operation", op),
	)
}

// statusOf names the outcome the way the family's metric label and log field
// expect — the same two words the other DB backends use.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// applyObservability attaches the access-log + operation-metric hook to client.
func applyObservability(client redis.UniversalClient) {
	client.AddHook(newObserveHook())
}

// observeHook emits a per-command access log and the family's duration /
// in-flight instruments around every Redis command and pipeline; it sits
// outside the resilience hook (see Client.Init) so one log line covers the
// whole retry loop.
type observeHook struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

var _ redis.Hook = (*observeHook)(nil)

func (h *observeHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *observeHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		op := cmd.FullName()
		if _, skip := skipOps[op]; skip {
			return next(ctx, cmd)
		}
		start := time.Now()
		inflight := inflightOf(op)
		h.active.Add(ctx, 1, inflight)
		err := next(ctx, cmd)
		h.record(ctx, op, argOf(cmd), start, nilAsSuccess(err), inflight)
		return err
	}
}

func (h *observeHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		inflight := inflightOf("pipeline")
		h.active.Add(ctx, 1, inflight)
		err := next(ctx, cmds)
		h.record(ctx, "pipeline", "", start, nilAsSuccess(err), inflight)
		return err
	}
}

// argOf picks the command's first argument — the key for every keyed Redis
// command — as the access-log argument, truncated so a large value cannot
// dominate the log line.
func argOf(cmd redis.Cmder) string {
	args := cmd.Args()
	if len(args) < 2 {
		return ""
	}
	return strutil.Truncate(fmt.Sprint(args[1]), 512)
}

// record emits one finished command's duration, balances the in-flight gauge,
// and writes its access-log entry. The log level carries the outcome: an error
// at Warn with the error field; a keyed success at Debug in the lazy
// `func() []log.Field` form (keyed commands are the common case and
// uninteresting until they fail, and the lazy form skips formatting when Debug
// is filtered); a keyless success at Info.
func (h *observeHook) record(ctx context.Context, op, arg string, start time.Time, err error, inflight metric.MeasurementOption) {
	status := statusOf(err)
	dur := float64(time.Since(start).Nanoseconds()) / 1e6
	h.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String("db.system", redisSystem),
		attribute.String("db.operation", op),
		attribute.String("status", status),
	))
	h.active.Add(ctx, -1, inflight)

	if err != nil {
		log.Warn(ctx, accessTag,
			log.String("db.operation", op),
			log.String("status", status),
			log.Float("duration_ms", dur),
			log.Any("error", err),
		)
		return
	}
	if arg != "" {
		log.Debug(ctx, accessTag, func() []log.Field {
			return []log.Field{
				log.String("db.operation", op),
				log.String("status", status),
				log.Float("duration_ms", dur),
				log.String("db.statement", arg),
			}
		})
		return
	}
	log.Info(ctx, accessTag,
		log.String("db.operation", op),
		log.String("status", status),
		log.Float("duration_ms", dur),
	)
}
