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

package StarterRedigo

import (
	"context"
	"fmt"
	"go-spring.org/stdlib/strutil"
	"strings"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
var (
	// accessTag is the static log tag for the redigo access log.
	accessTag = log.RegisterAppTag("redigo", "access")

	cmdTracer = otel.Tracer("go-spring.org/starter-redigo")
)

// skipOps are commands that skip instrumentation entirely (span + metric +
// log together): PING fires on every health probe and pool test-on-borrow,
// so instrumenting it is pure noise.
var skipOps = map[string]struct{}{"PING": {}}

// newDuration builds the db.client.operation.duration histogram from
// whatever meter provider is current — created per pool at construction,
// not at package init, so an SDK installed after this package's init still
// receives the records.
func newDuration() metric.Float64Histogram {
	h, _ := otel.Meter("go-spring.org/starter-redigo").Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of redis client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	return h
}

// observeInterceptor is the observe layer of the command chain: it starts a
// client span for the command (trace + duration metric + access log), runs
// next under it, and ends the span with the result. ctx is the span parent —
// the caller's context for DoContext (so the span links to the request trace
// and an attempt-timeout can interrupt it), background for the context-less
// paths. The span sits OUTSIDE the resilience layer, so one Execute (with any
// retries the policy drives) is covered by a single span. Skipped ops pass
// through untouched.
func observeInterceptor(duration metric.Float64Histogram) CommandInterceptor {
	return func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (reply interface{}, err error) {
			if _, skip := skipOps[strings.ToUpper(cmd)]; skip {
				return next(ctx, cmd, args)
			}
			start := time.Now()
			statement := summarizeCommand(cmd, args)
			ctx, span := cmdTracer.Start(ctx, cmd,
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(
					attribute.String("db.system", "redis"),
					attribute.String("db.operation", strings.ToLower(cmd)),
					attribute.String("db.statement", statement),
				))
			reply, err = next(ctx, cmd, args)
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
			}
			span.End()
			record(ctx, duration, cmd, statement, args, start, err)
			return reply, err
		}
	}
}

// record emits the duration metric and the access log for one finished
// command. The log level carries the outcome: an error at Warn, a success
// that names its key at Debug (lazy — the common case is uninteresting), and
// a keyless success (ECHO, FLUSHALL, SELECT, ...) at Info.
func record(ctx context.Context, duration metric.Float64Histogram, cmd, statement string, args []interface{}, start time.Time, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	dur := float64(time.Since(start).Nanoseconds()) / 1e6
	duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String("db.operation", strings.ToLower(cmd)),
		attribute.String("status", status),
	))

	common := func() []log.Field {
		return []log.Field{
			log.String("operation", cmd),
			log.String("statement", statement),
			log.Float("duration_ms", dur),
		}
	}
	switch {
	case err != nil:
		fields := append(common(), log.Any("error", err))
		log.Warn(ctx, accessTag, fields...)
	case len(args) > 0:
		log.Debug(ctx, accessTag, common)
	default:
		log.Info(ctx, accessTag, common()...)
	}
}

// summarizeCommand renders a short, loggable summary of the command — the
// command name plus the first argument (typically the key) — truncated on a
// rune boundary. The full argument list is intentionally not logged: keys are
// enough to locate an op, and values may be sensitive or large.
func summarizeCommand(cmd string, args []interface{}) string {
	if len(args) == 0 {
		return cmd
	}
	return strutil.Truncate(fmt.Sprintf("%s %v", cmd, args[0]), 512)
}
