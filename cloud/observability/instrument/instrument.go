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

// Package instrument is the neutral instrumentation plumbing shared by the
// outbound and inbound observers. It sits OUTSIDE the data-plane's direction
// trees because neither direction owns it: the meter scope, the tracer scope,
// the access-log tag and the small renderers are the same words on both sides.
//
// It imports no governance code (only log and the OTel attribute vocabulary), so
// either observer — or a test — can use it without pulling the other direction
// into its dependency graph.
package instrument

import (
	"context"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

const (
	// Scope is the OTel meter scope every signal in this family registers under,
	// shared by the outbound (resilience.client.*) and inbound
	// (resilience.server.*) families alike.
	//
	// The string deliberately still says "resilience": it is an observability
	// contract (dashboards and backends key off it), not a package path. Moving
	// the inbound half under inbound/ does not rename what it emits.
	Scope = "go-spring.org/cloud/runtime/dataplane/resilience"

	// ComponentName is the instrumentation component name every tracer in this
	// family reports under. Like Scope, it is kept byte-for-byte for backend
	// continuity.
	ComponentName = "go-spring.org/cloud/runtime/dataplane/observability"
)

// Tag is the static log tag for the resilience access log; the protected
// client's or server's identity is a log field, not part of the tag.
var Tag = log.RegisterAppTag("resilience", "")

// AttrsToLogFields renders attributes as log fields, so the log keys equal the
// metric labels' names and a dashboard's selector joins the two.
func AttrsToLogFields(attrs []attribute.KeyValue) []log.Field {
	fields := make([]log.Field, 0, len(attrs))
	for _, a := range attrs {
		key := string(a.Key)
		switch a.Value.Type() {
		case attribute.BOOL:
			fields = append(fields, log.Bool(key, a.Value.AsBool()))
		case attribute.INT64:
			fields = append(fields, log.Any(key, a.Value.AsInt64()))
		case attribute.FLOAT64:
			fields = append(fields, log.Float(key, a.Value.AsFloat64()))
		default:
			fields = append(fields, log.String(key, a.Value.AsString()))
		}
	}
	return fields
}

// DurMs renders a duration in milliseconds for the access log, the unit the log
// field name promises.
func DurMs(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

// ReportInstrumentError surfaces a failed instrument registration. A declared
// metric family that is not a legal OTel instrument name makes every instrument
// built from it unusable, and the SDK says so only through the error it returns
// — which the observers used to discard, so a typo in a declaration silently
// deleted that client's entire metric surface. It is loud (Warn) and happens
// once per family, since the instrument sets are memoized per prefix.
func ReportInstrumentError(family string, err error) {
	if err == nil {
		return
	}
	log.Warn(context.Background(), Tag,
		"observability: metric instruments could not be registered — a declared metric family is not a legal instrument name, so this operation's metrics are dropped",
		log.String("metric_family", family), log.Err(err))
}
