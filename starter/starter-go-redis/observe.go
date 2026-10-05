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

// observe.go declares what a redis command IS. The signals themselves — the
// span, the duration metrics, the access log — are emitted by the resilience
// layer, the single point on the command hook chain that sees a whole call
// (retries included). This file therefore holds no emission code: only the
// vocabulary that this starter alone knows, because only it knows these
// commands reach a redis backend.
//
// redisotel still supplies the connection-POOL metrics (an observable-gauge
// family, not per-call — see [instrument] in starter.go); its per-command span
// is gone, because it duplicated the call span the resilience layer now opens.

package StarterGoRedis

import (
	"context"
	"fmt"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"github.com/redis/go-redis/v9"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// redisSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const redisSystem = "redis"

// maxStatement bounds the command's argument captured as db.statement. A key
// can be long and a span or a log line has no use for all of it.
const maxStatement = 512

// accessTag is the static access-log tag for Redis commands, registered once at
// package init so logger config can address it (_app_redis_access). A tag must
// exist before the framework's first property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("redis", "access")

// skipOps are the commands that declare no identity, so nothing
// family-specific is emitted for them: the health-check PING (health/health.go)
// fires periodically per instance and would flood the log with uninteresting
// success lines. They still run under the resilience layer, which reports them
// as it reports any other call whose identity is undeclared.
var skipOps = map[string]struct{}{"ping": {}}

// operation is the semantic identity of one redis command. The command's full
// name (cmd.FullName(), e.g. "get") names the span and carries the family's
// db.operation label.
//
// The command's argument rides in Detail rather than Attrs: it is usually a
// key, drawn from an open set, so as a metric label one would multiply the
// series without bound. Detail reaches the span and the log — where a key is
// exactly what makes a line worth reading — and never a label. A command with
// no argument carries no detail at all, which is also what levelled its
// success log at Info.
func operation(cmd redis.Cmder) observability.Operation {
	op := cmd.FullName()
	o := observability.Operation{
		Name:   op,
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", redisSystem),
			attribute.String("db.operation", op),
		},
		LogTag: accessTag,
	}
	if arg := argOf(cmd); arg != "" {
		o.Detail = []attribute.KeyValue{
			attribute.String("db.statement", arg),
		}
	}
	return o
}

// pipelineOperation is the semantic identity of a pipelined batch: a pipeline
// is not one command and carries no single argument, so its name is the literal
// "pipeline" and it declares no Detail — which levels its success log at Info.
func pipelineOperation() observability.Operation {
	return observability.Operation{
		Name:   "pipeline",
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", redisSystem),
			attribute.String("db.operation", "pipeline"),
		},
		LogTag: accessTag,
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
	return strutil.Truncate(fmt.Sprint(args[1]), maxStatement)
}

// applyDeclaration attaches the declaration hook to client. It is added before
// the resilience hook (see [NewClient]) so the identity it puts on the ctx is
// already there when the resilience layer reads it and emits the call's span,
// metrics and access log.
func applyDeclaration(client redis.UniversalClient) {
	client.AddHook(&operationHook{})
}

// operationHook is the declaration layer of the command hook chain: it puts the
// command's identity on the context, which the resilience layer inside it reads
// to emit the span, the metrics and the access log. A skipped op is forwarded
// untouched, declaring no Operation — the resilience layer then reports it as
// any other call whose identity is undeclared.
type operationHook struct{}

var _ redis.Hook = (*operationHook)(nil)

func (h *operationHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *operationHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if _, skip := skipOps[cmd.FullName()]; skip {
			return next(ctx, cmd)
		}
		return next(observability.WithOperation(ctx, operation(cmd)), cmd)
	}
}

func (h *operationHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		return next(observability.WithOperation(ctx, pipelineOperation()), cmds)
	}
}
