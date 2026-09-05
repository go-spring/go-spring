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

// observe.go is the module-local access log for Redis commands. It is
// deliberately log-only: spans and metrics come from redisotel (installed by
// instrument() in starter.go), so this hook opens no span and records no
// metric itself — emitting them here would duplicate redisotel's signals.
package StarterGoRedis

import (
	"context"
	"fmt"
	"go-spring.org/stdlib/strutil"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/log"
)

// accessTag is the static access-log tag for Redis commands, registered once
// at package init so logger config can address it (_app_redis_access).
var accessTag = log.RegisterAppTag("redis", "access")

// skipOps are the commands the access log suppresses entirely. Local decision,
// not config: the health-check PING (health/health.go) fires periodically per
// instance and would flood the log with uninteresting success lines.
var skipOps = map[string]struct{}{"ping": {}}

// applyObservability attaches the access-log hook to client. See the package
// comment in observe.go: it only drives the log path.
func applyObservability(client redis.UniversalClient) {
	client.AddHook(&observeHook{})
}

// observeHook emits a per-command access log around every Redis command and
// pipeline; it sits outside the resilience hook (see Client.Init) so one log
// line covers the whole retry loop.
type observeHook struct{}

var _ redis.Hook = (*observeHook)(nil)

func (h *observeHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *observeHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		op := cmd.FullName()
		if _, skip := skipOps[op]; skip {
			return next(ctx, cmd)
		}
		start := time.Now()
		err := next(ctx, cmd)
		record(ctx, op, argOf(cmd), start, nilAsSuccess(err))
		return err
	}
}

func (h *observeHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		record(ctx, "pipeline", "", start, nilAsSuccess(err))
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

// record writes one access-log entry. The log level carries the outcome: an
// error at Warn with the error field; a keyed success at Debug in the lazy
// `func() []log.Field` form (keyed commands are the common case and
// uninteresting until they fail, and the lazy form skips formatting when Debug
// is filtered); a keyless success at Info.
func record(ctx context.Context, op, arg string, start time.Time, err error) {
	if err != nil {
		log.Warn(ctx, accessTag,
			log.String("db.operation", op),
			log.String("status", "error"),
			log.Float("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
			log.Any("error", err),
		)
		return
	}
	if arg != "" {
		log.Debug(ctx, accessTag, func() []log.Field {
			return []log.Field{
				log.String("db.operation", op),
				log.String("status", "ok"),
				log.Float("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
				log.String("db.statement", arg),
			}
		})
		return
	}
	log.Info(ctx, accessTag,
		log.String("db.operation", op),
		log.String("status", "ok"),
		log.Float("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
	)
}
