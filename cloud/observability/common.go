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

package observability

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/chain"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// resilienceTag is the static log tag for the resilience access log; the
// protected client's system is a log field, not part of the tag.
var resilienceTag = log.RegisterAppTag("resilience", "")

// executorScope names the OTel scope this package's spans and instruments
// register under, shared by the outbound (resilience.client.*) and inbound
// (resilience.server.*) families alike.
const executorScope = "go-spring.org/cloud/governance/resilience"

// ResetExecutorInstruments drops the memoized instrument sets so the next use
// resolves fresh ones. It exists for tests that install their own
// MeterProvider: the sets are process-wide and resolved once, so a test running
// after one that already resolved them would otherwise keep reporting into the
// earlier provider.
//
// It covers both directions — the always-on client and server sets and the
// per-prefix operation sets — since a MeterProvider swap invalidates all of them
// together.
func ResetExecutorInstruments() {
	clientInstruments = sync.OnceValue(buildClientInstruments)
	serverInstruments = sync.OnceValue(buildServerInstruments)
	resetOperationInstruments()
}

// resetOperationInstruments drops the memoized per-prefix sets so the next use
// resolves fresh ones, for the same reason [ResetExecutorInstruments] does.
func resetOperationInstruments() {
	opInstruments.mu.Lock()
	defer opInstruments.mu.Unlock()
	opInstruments.m = map[string]*opInstrumentSet{}
	opServerInstruments.mu.Lock()
	defer opServerInstruments.mu.Unlock()
	opServerInstruments.m = map[string]*opServerInstrumentSet{}
}

// successQuiet reports whether a successful call's access line is logged at
// Debug (true) or Info (false).
//
// An UNDECLARED call is this layer's historical quiet case: it has always logged
// success at Debug, and a client that declares nothing (redigo's skipped PING,
// say) declares nothing precisely to keep the noise down — so it stays quiet.
// A declared call says so itself, via [chain.Observe.Log.QuietSuccess]: a producer
// whose calls carry high-frequency per-call detail sets it, one whose success is
// worth a line does not.
func successQuiet(hasOp bool, op chain.Operation) bool {
	if !hasOp {
		return true
	}
	return op.Observe.Log.QuietSuccess
}

// attrsToLogFields renders the operation's attributes as log fields, so the log
// keys equal the metric labels' names and a dashboard's selector joins the two.
func attrsToLogFields(attrs []attribute.KeyValue) []log.Field {
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

// durMs renders a duration in milliseconds for the access log, the unit the log
// field name promises.
func durMs(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

// reportInstrumentError surfaces a failed instrument registration. A declared
// metric family that is not a legal OTel instrument name makes every instrument
// built from it unusable, and the SDK says so only through the error it returns
// — which this package used to discard, so a typo in a declaration silently
// deleted that client's entire metric surface. It is loud (Warn) and happens
// once per family, since the instrument sets are memoized per prefix.
func reportInstrumentError(family string, err error) {
	if err == nil {
		return
	}
	log.Warn(context.Background(), resilienceTag,
		"observability: metric instruments could not be registered — a declared metric family is not a legal instrument name, so this operation's metrics are dropped",
		log.String("metric_family", family), log.Err(err))
}

// ClassifyStatus lives in the chain package, beside the sentinels whose
// errors it classifies; this emitter reads it from there (chain.ClassifyStatus)
// rather than owning a word that is really the governance layer's.
