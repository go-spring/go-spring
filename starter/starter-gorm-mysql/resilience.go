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

package StarterGormMySql

import (
	"context"
	"errors"
	"sync"

	"go-spring.org/spring/resilience"
	"gorm.io/gorm"
)

// resilienceExecs tracks the resilience executor attached to each *gorm.DB, so
// destroyClient can Close it (releasing any background resources of a
// production driver). The key is the *gorm.DB; only clients with resilience
// enabled appear here.
var resilienceExecs sync.Map // *gorm.DB -> resilience.Executor

// applyResilience builds an executor from the configured driver and wraps every
// standard gorm processor with it, unless resilience is disabled. This is the
// gorm seam of stdlib/resilience: the same backend-neutral Executor that
// starter-oauth2-client drives through an http.RoundTripper and starter-go-redis
// drives through a redis.Hook is here driven through gorm's callback chain,
// proving the core is reused while only the adapter differs per library.
func applyResilience(c Config, db *gorm.DB) error {
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
	resource := resourceLabel(c)

	// Each processor's final registered callback is Replaced with a wrapper that
	// runs the original under the executor. This covers every SQL op gorm emits.
	type pStep struct {
		p    interface {
			Get(string) func(*gorm.DB)
			Replace(string, func(*gorm.DB)) error
		}
		name string
	}
	steps := []pStep{
		{db.Callback().Create(), "gorm:create"},
		{db.Callback().Query(), "gorm:query"},
		{db.Callback().Update(), "gorm:update"},
		{db.Callback().Delete(), "gorm:delete"},
		{db.Callback().Row(), "gorm:row"},
		{db.Callback().Raw(), "gorm:raw"},
	}
	for _, s := range steps {
		orig := s.p.Get(s.name)
		if orig == nil {
			continue
		}
		name := s.name
		fn := orig
		wrapped := func(tx *gorm.DB) {
			err := runGuard(tx.Statement.Context, exec, resource, func() error {
				fn(tx)
				return tx.Error
			})
			if err != nil && (errors.Is(err, resilience.ErrRateLimited) || errors.Is(err, resilience.ErrCircuitOpen)) {
				// Rejected by limiter/breaker: surface the rejection on tx.Error.
				_ = tx.AddError(err)
			}
		}
		if err := s.p.Replace(name, wrapped); err != nil {
			return err
		}
	}

	resilienceExecs.Store(db, exec)
	return nil
}

// closeResilience closes and forgets the executor behind db, if any.
func closeResilience(db *gorm.DB) {
	if v, ok := resilienceExecs.LoadAndDelete(db); ok {
		_ = v.(resilience.Executor).Close()
	}
}

// runGuard executes call under exec, translating rejections but treating
// gorm.ErrRecordNotFound as success — the DB analog of redis.Nil: "no rows" is
// a normal outcome, not a fault, so it must never trip the breaker.
func runGuard(ctx context.Context, exec resilience.Executor, resource string, call func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var callErr error
	execErr := exec.Execute(ctx, resource, func(context.Context) error {
		callErr = call()
		if callErr != nil && !errors.Is(callErr, gorm.ErrRecordNotFound) {
			return callErr // a real failure feeds the breaker/retry
		}
		return nil // success or "no rows"
	})
	if execErr != nil {
		if errors.Is(execErr, resilience.ErrRateLimited) || errors.Is(execErr, resilience.ErrCircuitOpen) {
			return execErr
		}
		// A real op error propagated through the executor; callErr already holds it.
		return callErr
	}
	return callErr
}

// resourceLabel derives a stable, human-readable resilience resource key for a
// client, so limiter and breaker state is scoped per DB instance rather than
// per statement.
func resourceLabel(c Config) string {
	switch {
	case c.ServiceName != "":
		return "gorm:mysql:" + c.ServiceName
	case c.Addr != "":
		return "gorm:mysql:" + c.Addr
	default:
		return "gorm:mysql"
	}
}
