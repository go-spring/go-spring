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

package lock

import "time"

// Timing defaults come from three layers, and the order matters. Resolve is the
// single place that order is encoded, so every backend composes them the same
// way:
//
//	per-acquisition Option  >  starter Defaults  >  package default
//
// A starter (e.g. spring.lock.<name>.ttl) feeds a [Defaults] built from its
// bound config; a per-call [WithTTL]/[WithRenewInterval]/[WithRetryInterval]
// always wins; whatever is still unset falls through to the package default
// documented on [Options]. Backends that expose no starter-level timing knobs
// pass a zero [Defaults], which makes Resolve behave exactly like [Apply].
//
// Why this exists: [Apply] normalizes a zero [Options.TTL] to 30s before a
// backend ever sees it, so a backend cannot tell "caller asked for 30s" from
// "caller passed nothing" — which used to leave starter-level TTL config either
// dead (consul) or a fixed override that silently ignored per-call opts (etcd).
// Resolve injects the starter default before that normalization, restoring the
// intended precedence.

// Defaults are starter/backend-level timing values layered beneath the
// per-acquisition opts. A zero field means "no opinion — use the package
// default", so a backend only populates the fields its config actually exposes.
type Defaults struct {
	TTL           time.Duration
	RenewInterval time.Duration
	RetryInterval time.Duration
}

// Resolve normalizes opts with d layered beneath them. Backends call it at the
// top of Acquire/TryAcquire instead of [Apply], so starter config and per-call
// opts compose identically across every backend. See the package-level comment
// for the precedence rules.
//
// RenewInterval keeps its special semantics through the layering: a positive d
// fills a caller-set zero, while a caller's explicit negative (auto-renew
// disabled) is non-zero and is therefore preserved.
func Resolve(d Defaults, opts ...Option) Options {
	var o Options
	for _, fn := range opts {
		fn(&o)
	}
	// Inject starter defaults only for fields the caller left at their "no
	// opinion" zero. Doing this before normalize keeps an explicit zero/negative
	// (e.g. RenewInterval < 0 to disable renew) intact.
	if o.TTL == 0 && d.TTL > 0 {
		o.TTL = d.TTL
	}
	if o.RenewInterval == 0 && d.RenewInterval != 0 {
		o.RenewInterval = d.RenewInterval
	}
	if o.RetryInterval == 0 && d.RetryInterval > 0 {
		o.RetryInterval = d.RetryInterval
	}
	o.normalize()
	return o
}
