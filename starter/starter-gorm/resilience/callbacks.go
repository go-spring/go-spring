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

// Package gormresilience is the shared gorm resilience adapter. It replaces
// gorm's standard create/query/update/delete/row/raw callback processors with
// wrappers that run each operation under one [resilience.ClientExecutor], so every
// gorm dialect starter (mysql, postgres, clickhouse, sqlserver) shares one
// implementation instead of copy-pasting the ~100-line callback chain each.
//
// It is the gorm seam of resilience: the same backend-neutral Executor that
// other adapters drive through a redis.Hook, an http.RoundTripper or a grpc
// interceptor is here driven through gorm's callback chain. The executor is
// built per-instance by the starter (which knows its Config + driver + the
// optional observe-resilience wrapper); this package only attaches it.
//
// gorm.ErrRecordNotFound is treated as success — "no rows" is a normal outcome,
// not a fault, so it must not trip the breaker (the DB analog of redis.Nil).
package gormresilience

import (
	"context"

	"go-spring.org/cloud/resilience"
	"gorm.io/gorm"
)

// callbackProcessor is the shape of every gorm callback stage (Create/Query/...
// /Raw): it looks up a registered callback by name and replaces it.
type callbackProcessor interface {
	Get(string) func(*gorm.DB)
	Replace(string, func(*gorm.DB)) error
}

// ApplyCallbacks replaces gorm's six standard processors with wrappers that run
// each operation under exec, scoped to service. A [gorm.ErrRecordNotFound] from
// the op is treated as success; resilience rejections (ErrRateLimited /
// ErrCircuitOpen / ErrBulkheadFull) surface on tx.Error so the caller sees them.
// It is a no-op for any processor gorm has not registered (Get returns nil).
func ApplyCallbacks(db *gorm.DB, exec resilience.ClientExecutor, service string) error {
	steps := []struct {
		p    callbackProcessor
		name string
	}{
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
		fn := orig
		wrapped := func(tx *gorm.DB) {
			err := runGuard(tx.Statement.Context, exec, service, func() error {
				fn(tx)
				return tx.Error
			})
			// Propagate the guard's error onto tx.Error when it is a protection
			// rejection OR when fn never set tx.Error — the latter happens when a
			// fault injector short-circuited the attempt before fn ran, leaving the
			// injected error otherwise dropped. When fn ran and failed, tx.Error is
			// already set and is left untouched.
			if err != nil && (resilience.IsRejection(err) || tx.Error == nil) {
				_ = tx.AddError(err)
			}
		}
		if err := s.p.Replace(s.name, wrapped); err != nil {
			return err
		}
	}
	return nil
}

// runGuard executes call under exec via [resilience.Run], treating
// gorm.ErrRecordNotFound ("no rows") as success so it never trips the breaker.
// A real op error propagates through the executor (feeding retry/breaker); the
// rejection sentinels are returned as-is so ApplyCallbacks' wrapper can put them
// on tx.Error.
func runGuard(ctx context.Context, exec resilience.ClientExecutor, service string, call func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := resilience.Run(ctx, exec,
		func(context.Context) (struct{}, error) { return struct{}{}, call() },
		resilience.Tolerate(gorm.ErrRecordNotFound))
	return err
}
