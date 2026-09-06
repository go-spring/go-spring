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

package luohua

import (
	"context"

	"go-spring.org/log"
)

// applyObservability re-baselines the log context hook (the log.FieldsFromContext
// seam) so every log event prints luohua's business context (tenant, user, ...)
// on top of whatever the framework already surfaces.
//
// The hook is composed rather than replaced: the previous hook (e.g. the one
// that prints trace_id) is captured and called first, so luohua adds fields
// instead of clobbering them. Note the composition is best-effort on module
// ordering — if a later module installs a log hook after luohua runs, that hook
// wins; an application needing strict ordering composes luohua's fields itself.
func applyObservability(c ObservabilityConfig) error {
	if len(c.Fields) == 0 {
		return nil
	}
	fields := append([]string(nil), c.Fields...)
	prev := log.FieldsFromContext
	log.FieldsFromContext = func(ctx context.Context) []log.Field {
		out := []log.Field{}
		if prev != nil {
			out = append(out, prev(ctx)...)
		}
		for _, name := range fields {
			if v := carryHeader(ctx, name); v != "" {
				out = append(out, log.String(name, v))
			}
		}
		return out
	}
	return nil
}
