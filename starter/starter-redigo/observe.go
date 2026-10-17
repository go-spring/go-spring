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
	"fmt"
	"strings"

	"go-spring.org/stdlib/strutil"

	"go-spring.org/cloud/runtime/dataplane/chain"
	"go-spring.org/log"
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
func operation(cmd string, args []interface{}) chain.Operation {
	stmt := ""
	if len(args) > 0 {
		stmt = summarizeCommand(cmd, args)
	}
	op := chain.DBOp(redisSystem, strings.ToLower(cmd), stmt)
	// The span is named after the command as written ("GET"), while the
	// db.operation label carries it lowercased, the family's convention.
	op.Request.Action = cmd
	op.Observe.Log.Tag = accessTag
	return op
}

// opFor builds the operation for one command, or the zero Operation when the
// command declares no identity (see skipOps) — an "undeclared" call the
// resilience layer still runs and reports as such. It is the single place the
// skip decision is made; the resilience interceptor (see conn.go) calls it to
// build the op it passes explicitly to the executor.
func opFor(cmd string, args []interface{}) chain.Operation {
	if _, skip := skipOps[strings.ToUpper(cmd)]; skip {
		return chain.Operation{}
	}
	return operation(cmd, args)
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
