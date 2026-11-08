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
// wrappers that run each operation under one [chain.Executor], so every
// gorm dialect starter (mysql, postgres, clickhouse, sqlserver) shares one
// implementation instead of copy-pasting the ~100-line callback chain each.
//
// It is the gorm seam of resilience: the same backend-neutral chain.Executor that
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
	"errors"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
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
//
// opFor supplies the [chain.Operation] the wrapped processor is about to run —
// its identity, built where the operation's kind and its dialect are known (see
// the observe plugin's Plugin.Operation). It is asked once per call, at the
// wrap point, and the result is passed EXPLICITLY to the executor rather than
// carried on the context. A nil opFor (observe disabled) runs every call with
// the zero Operation, so the resilience layer falls back to its generic
// signals.
func ApplyCallbacks(db *gorm.DB, exec chain.Executor, service string, opFor func(*gorm.DB, string) chain.Operation) error {
	// The dialect's own error translator answers "is this a constraint verdict"
	// without gorm's global TranslateError: we call Translate for CLASSIFICATION
	// only, so the error returned to the caller stays the driver's own. A custom
	// dialect that does not implement it classifies nothing and every error
	// remains a downstream fault.
	translator, _ := db.Dialector.(gorm.ErrorTranslator)
	steps := []struct {
		p    callbackProcessor
		name string
		kind string
	}{
		{db.Callback().Create(), "gorm:create", "create"},
		{db.Callback().Query(), "gorm:query", "query"},
		{db.Callback().Update(), "gorm:update", "update"},
		{db.Callback().Delete(), "gorm:delete", "delete"},
		{db.Callback().Row(), "gorm:row", "row"},
		{db.Callback().Raw(), "gorm:raw", "raw"},
	}
	for _, s := range steps {
		orig := s.p.Get(s.name)
		if orig == nil {
			continue
		}
		fn := orig
		kind := s.kind
		wrapped := func(tx *gorm.DB) {
			// Build the call's identity where the processor (and so its kind and
			// dialect) is known, then pass it explicitly to the executor.
			var op chain.Operation
			if opFor != nil {
				op = opFor(tx, kind)
			}
			// The service is this executor's label; the operation states it so a
			// layer outside the executor (the fault layer) can match on it too.
			op.Service = service
			err := runGuard(tx.Statement.Context, exec, op, translator, func() error {
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

// runGuard executes call under exec via [resilience.Run] for the operation op,
// treating two kinds of error as normal outcomes: gorm.ErrRecordNotFound ("no
// rows") and a constraint verdict — the database definitively rejected the
// statement, so the dependency is healthy. Neither trips the breaker nor
// triggers a retry, while both are returned to the caller verbatim. A real op
// error propagates through the executor (feeding retry/breaker); the rejection
// sentinels are returned as-is so ApplyCallbacks' wrapper can put them on
// tx.Error.
//
// translator is the dialect's ErrorTranslator (nil when the dialect has none).
// It is used ONLY to classify: unlike gorm's global TranslateError, it does not
// rewrite the error the caller receives.
//
// No retry class is declared here: gorm drivers expose no common transient
// vocabulary (a constraint verdict is permanent, while a connection failure is
// driver-specific), and database/sql already retries driver.ErrBadConn
// internally. A retry schedule configured for a gorm label therefore still runs
// once, which the document owner controls.
func runGuard(ctx context.Context, exec chain.Executor, op chain.Operation,
	translator gorm.ErrorTranslator, call func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	opts := []resilience.Option{resilience.Tolerate(gorm.ErrRecordNotFound)}
	if translator != nil {
		opts = append(opts, resilience.TolerateFunc(func(err error) bool {
			return isConstraintVerdict(translator, err)
		}))
	}
	_, err := resilience.Run(ctx, exec, op,
		func(context.Context) (struct{}, error) { return struct{}{}, call() },
		opts...)
	return err
}

// isConstraintVerdict reports whether the dialect translates err to one of
// gorm's constraint sentinels. The translation is done on the SIDE — the
// original error is what the caller keeps — which is why this asks the dialect
// rather than enabling gorm's TranslateError globally: flipping that option
// would replace every matching error with the sentinel, breaking callers that
// inspect the driver's own type.
func isConstraintVerdict(translator gorm.ErrorTranslator, err error) bool {
	translated := translator.Translate(err)
	return errors.Is(translated, gorm.ErrDuplicatedKey) ||
		errors.Is(translated, gorm.ErrForeignKeyViolated) ||
		errors.Is(translated, gorm.ErrCheckConstraintViolated)
}
