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

package resilience

import (
	"context"
	"sync"

	"go-spring.org/log"
)

// warnedUnmanaged records the clients already told that they run unmanaged, so
// the warning is one per client rather than one per call.
var warnedUnmanaged sync.Map

// Unmanaged returns the executor a client runs on until — and unless — it is put
// under governance.
//
// It observes: the client is traced and measured exactly as a governed one is,
// which is what a client assembled by hand had before emission moved onto this
// chain. And it warns, once per client, because the client is running with no
// rate limit, no circuit breaker and no retry, and that absence is otherwise
// completely invisible — a client that silently skips its protection looks
// identical to a protected one until the downstream falls over.
//
// It is what a zero-valued governance bundle degrades to (see each client
// starter's [Deps]): assembling through the container hands the client a real
// executor instead, and nothing else changes.
func Unmanaged(system, service string) ClientExecutor {
	return &unmanagedExecutor{
		inner:   WrapClientExecutor(noopClientExecutor{}, system, service),
		service: service,
	}
}

// unmanagedExecutor is [Unmanaged]'s executor: it delegates to an observe-only
// wrapper over the pass-through, warning once on the way through.
type unmanagedExecutor struct {
	inner   ClientExecutor
	service string
}

func (e *unmanagedExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	if _, seen := warnedUnmanaged.LoadOrStore(e.service, struct{}{}); !seen {
		log.Warnf(ctx, log.TagAppDef,
			"resilience: client %q runs unmanaged — no rate limit, circuit breaker or retry applies to it; "+
				"it is observed only (assemble it through the container to govern it)", e.service)
	}
	return e.inner.Execute(ctx, fn)
}

func (e *unmanagedExecutor) Close() error                 { return e.inner.Close() }
func (e *unmanagedExecutor) Refresh(p ClientPolicy) error { return e.inner.Refresh(p) }
