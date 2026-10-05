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
// layer, the single point on the command chain that sees a whole call (retries
// included). This file therefore holds no emission code: only the vocabulary
// that this starter alone knows, because only it knows these commands reach a
// redis backend.

package StarterRedigo

import (
	"context"
	"fmt"
	"strings"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// accessTag is the static log tag for the redigo access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("redigo", "access")

// skipOps are commands that declare no identity, so nothing family-specific is
// emitted for them: PING fires on every health probe and pool test-on-borrow,
// so instrumenting it is pure noise. They still run under the resilience layer,
// which reports them as it reports any other call whose identity is undeclared.
var skipOps = map[string]struct{}{"PING": {}}

// redisSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const redisSystem = "redis"

// maxStatement bounds the command summary captured as db.statement.
const maxStatement = 512

// operation is the semantic identity of one redis command. The span is named
// after the command as written ("GET"); the db.operation label carries it
// lowercased, the family's convention.
//
// The command summary rides in Detail rather than Attrs: its first argument is
// usually a key, drawn from an open set, so as a metric label it would multiply
// the series without bound. A command with no arguments carries no detail at
// all, which is also what levelled its success log at Info.
func operation(cmd string, args []interface{}) observability.Operation {
	op := observability.Operation{
		Name:   cmd,
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", redisSystem),
			attribute.String("db.operation", strings.ToLower(cmd)),
		},
		LogTag: accessTag,
	}
	if len(args) > 0 {
		op.Detail = []attribute.KeyValue{
			attribute.String("db.statement", summarizeCommand(cmd, args)),
		}
	}
	return op
}

// operationInterceptor is the declaration layer of the command chain: it puts
// the command's identity on the context, which the resilience layer inside it
// reads to emit the span, the metrics and the access log. A skipped op is
// forwarded untouched.
func operationInterceptor() CommandInterceptor {
	return func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
			if _, skip := skipOps[strings.ToUpper(cmd)]; skip {
				return next(ctx, cmd, args)
			}
			return next(observability.WithOperation(ctx, operation(cmd, args)), cmd, args)
		}
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
	return strutil.Truncate(fmt.Sprintf("%s %v", cmd, args[0]), maxStatement)
}
