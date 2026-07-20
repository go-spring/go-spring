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

package StarterGoRedis

import (
	"context"
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/resilience"
)

// resilienceExecs tracks the resilience executor attached to each client, so the
// destructors can Close it (releasing any background resources of a production
// driver). The key is the client value; only clients with resilience enabled
// appear here.
var resilienceExecs sync.Map // redis.UniversalClient -> resilience.Executor

// applyResilience builds an executor from the configured driver and attaches it
// to client as a redis.Hook, unless resilience is disabled. This is the go-redis
// seam of stdlib/resilience: the same backend-neutral Executor that
// starter-oauth2-client drives through an http.RoundTripper is here driven
// through redis's per-command ProcessHook, proving the core is reused while only
// the adapter differs per library.
func applyResilience(c Config, client redis.UniversalClient) error {
	if !c.Resilience.Enabled {
		return nil
	}
	drv, err := resilience.MustGetDriver(c.Resilience.Driver)
	if err != nil {
		return err
	}
	exec, err := drv.NewExecutor(c.Resilience.policy())
	if err != nil {
		return err
	}
	client.AddHook(&resilienceHook{exec: exec, resource: resourceLabel(c)})
	resilienceExecs.Store(client, exec)
	return nil
}

// closeResilience closes and forgets the executor behind client, if any.
func closeResilience(client redis.UniversalClient) {
	if v, ok := resilienceExecs.LoadAndDelete(client); ok {
		_ = v.(resilience.Executor).Close()
	}
}

// resourceLabel derives a stable, human-readable resilience resource key for a
// client, so limiter and breaker state is scoped per Redis instance rather than
// per command. It falls back across the mode-specific address fields.
func resourceLabel(c Config) string {
	switch {
	case c.ServiceName != "":
		return "redis:" + c.ServiceName
	case c.MasterName != "":
		return "redis:" + c.MasterName
	case c.Addr != "":
		return "redis:" + c.Addr
	case len(c.Addrs) > 0:
		return "redis:" + c.Addrs[0]
	default:
		return "redis"
	}
}

// resilienceHook routes every Redis command (and pipeline) through the executor.
// DialHook is left untouched — connection establishment is discovery's concern,
// not the command-level protection we add here.
type resilienceHook struct {
	exec     resilience.Executor
	resource string
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
// under the policy, translating rejections into the command error and — crucially
// — treating redis.Nil (a cache miss / "key not found") as a success so it never
// trips the circuit breaker.
func (h *resilienceHook) run(ctx context.Context, setErr func(error), call func(context.Context) error) error {
	var callErr error
	execErr := h.exec.Execute(ctx, h.resource, func(ctx context.Context) error {
		callErr = call(ctx)
		if callErr != nil && !errors.Is(callErr, redis.Nil) {
			return callErr // a real failure feeds the breaker/retry
		}
		return nil // success or cache miss
	})
	if execErr != nil {
		if errors.Is(execErr, resilience.ErrRateLimited) || errors.Is(execErr, resilience.ErrCircuitOpen) {
			// Rejected before the command ran: surface the rejection to the caller.
			setErr(execErr)
			return execErr
		}
		// A real command error propagated through the executor; it is already the
		// callErr recorded on the command by go-redis.
		return callErr
	}
	return callErr
}
