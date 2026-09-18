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
	"sync"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// observabilityFields is the configured set of context fields luohua surfaces,
// read live because configuration arrives at module-run time.
var observabilityFields = struct {
	sync.RWMutex
	names []string
}{}

func setObservabilityFields(names []string) {
	observabilityFields.Lock()
	observabilityFields.names = names
	observabilityFields.Unlock()
}

// annotate attaches the configured luohua business context to ctx, as log
// fields and as span attributes, so both signals carry it from the point the
// headers were extracted onward.
//
// It runs where the values enter the process -- at the end of the propagator's
// Extract, which already has the carrier and has just stored each value in the
// context. That is the one place that needs to know: everything below it, the
// framework's own log lines and spans included, picks the context up for free.
//
// This replaced a log.FieldsFromContext hook. The hook was a pull that ran on
// every log line and read the same values back out of the context, which meant
// (a) it only ever covered logs, (b) it occupied the single global hook slot,
// so a second contributor had to compose with it by capturing and re-wrapping
// it -- order-dependent, as the hook's own comment admitted. Pushing at the
// boundary has neither problem, and covers spans as well.
//
// The field name is used verbatim as both the log key and the span attribute
// key, so what is configured is what appears in both.
func annotate(ctx context.Context) context.Context {
	observabilityFields.RLock()
	names := observabilityFields.names
	observabilityFields.RUnlock()
	if len(names) == 0 {
		return ctx
	}

	fields := make([]log.Field, 0, len(names))
	attrs := make([]attribute.KeyValue, 0, len(names))
	for _, name := range names {
		v := carryHeader(ctx, name)
		if v == "" {
			continue
		}
		fields = append(fields, log.String(name, v))
		attrs = append(attrs, attribute.String(name, v))
	}
	if len(fields) == 0 {
		return ctx
	}
	return observability.WithContextAttributes(log.WithFields(ctx, fields...), attrs...)
}

// applyObservability records which business context fields luohua surfaces on
// logs and spans. An empty list leaves both to whatever else is configured.
func applyObservability(c ObservabilityConfig) error {
	setObservabilityFields(c.Fields)
	return nil
}
