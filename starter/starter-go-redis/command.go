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

// command.go is the "command seam" concept of this starter: the go-redis
// [redis.Hook] layer that protects each command, mirroring starter-redigo's
// conn.go. Two hooks ride the client's hook chain (FIFO, first added outermost):
//
//	operationHook   — the declaration layer: puts the command's identity on the
//	                  ctx (see observe.go); emits nothing.
//	resilienceHook  — the breaker/retry/rate-limit executor (innermost), which
//	                  is also the single emitter of the call's span, metrics
//	                  and access log.
//
// Their relative order is established at construction (NewClient adds the
// declaration, then the resilience hook) and is a
// semantic contract: the declaration sits outside the breaker, so the identity
// reaches the resilience layer and one log line covers the whole retry loop.
package StarterGoRedis

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/resilience"
)

// resilienceHook routes every Redis command (and pipeline) through the executor.
// DialHook is left untouched — connection establishment is discovery's concern,
// not the command-level protection we add here.
type resilienceHook struct {
	exec         resilience.ClientExecutor
	serviceLabel string
}

var _ redis.Hook = (*resilienceHook)(nil)

func (h *resilienceHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *resilienceHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		return h.guard(ctx, cmd, func(ctx context.Context) error {
			return next(ctx, cmd)
		})
	}
}

func (h *resilienceHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		var setErr = func(err error) {
			for _, cmd := range cmds {
				cmd.SetErr(err)
			}
		}
		return h.run(ctx, setErr, func(ctx context.Context) error {
			return next(ctx, cmds)
		})
	}
}

// guard runs a single command through the executor, tagging the command with a
// rejection error when the limiter or breaker short-circuits it.
func (h *resilienceHook) guard(ctx context.Context, cmd redis.Cmder, call func(context.Context) error) error {
	return h.run(ctx, cmd.SetErr, call)
}

// run is the shared body for both command and pipeline hooks. It executes call
// under the policy via [resilience.Run], treating redis.Nil (a cache miss /
// "key not found") as a success so it never trips the circuit breaker.
//
// The setErr side-channel is why go-redis keeps a thin wrapper rather than
// calling resilience.Run directly: a normal downstream failure is already
// recorded on the command(s) by go-redis itself, and for a pipeline the
// per-command errors must be preserved, not overwritten with the aggregate
// error. So setErr fires only when the command never actually ran — a
// resilience rejection or an injected fault — where go-redis had no chance to
// record anything. callErr is tracked for that distinction.
func (h *resilienceHook) run(ctx context.Context, setErr func(error), call func(context.Context) error) error {
	var callErr error
	_, err := resilience.Run(ctx, h.exec,
		func(actx context.Context) (struct{}, error) {
			callErr = call(actx)
			return struct{}{}, callErr
		}, resilience.Tolerate(redis.Nil))
	if err != nil && callErr == nil {
		setErr(err)
	}
	return err
}
