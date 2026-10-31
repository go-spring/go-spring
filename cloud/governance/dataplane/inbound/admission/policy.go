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

package admission

import (
	"time"

	"go-spring.org/stdlib/errutil"
)

// Policy describes the protection wanted for one inbound server instance. It is
// the server-side half of the governance document and is deliberately different
// from the client's policy type: retry has no representation here, and there is
// no endpoint selection.
//
// Every field is zero-value-safe: an all-zero Policy configures nothing, and a
// Guard built from it admits every request with a no-op Done.
type Policy struct {
	// RateLimit caps inbound throughput in requests per second. 0 disables the
	// rate-limit stage.
	RateLimit float64 `value:"${rate-limit:=0}" expr:"$ >= 0"`

	// Burst is how many requests may exceed RateLimit momentarily; ignored when
	// RateLimit is 0.
	Burst int `value:"${burst:=0}" expr:"$ >= 0"`

	// RateLimitMaxWait is how long an over-limit request may wait for a token
	// before being refused. 0 refuses immediately. Ignored when RateLimit is 0.
	RateLimitMaxWait time.Duration `value:"${rate-limit-max-wait:=0}" expr:"int($) >= 0"`

	// Algorithm selects the counting scheme: empty means token bucket,
	// "sliding-window" the other. Ignored when RateLimit is 0.
	Algorithm string `value:"${algorithm:=}" expr:"$ == '' || $ in ['token-bucket', 'sliding-window']"`

	// Window is the rolling interval the sliding-window algorithm counts over;
	// defaults to one second. Ignored by the token bucket.
	Window time.Duration `value:"${window:=0}" expr:"int($) >= 0"`

	// RateLimitWarmUpDuration ramps the limit up from a cold start over this
	// duration. 0 disables warm-up.
	RateLimitWarmUpDuration time.Duration `value:"${rate-limit-warm-up-duration:=0}" expr:"int($) >= 0"`

	// WarmUpColdFactor shapes the ramp-in; 0 means the engine's default.
	WarmUpColdFactor int `value:"${warm-up-cold-factor:=0}" expr:"$ >= 0"`

	// MaxConcurrent caps how many requests the instance handles at the same
	// time (the bulkhead). 0 disables it.
	MaxConcurrent int `value:"${max-concurrent:=0}" expr:"$ >= 0"`

	// BulkheadQueueDuration is how long a request finding the bulkhead full may
	// wait for a slot before being refused. 0 refuses immediately.
	BulkheadQueueDuration time.Duration `value:"${bulkhead-queue-duration:=0}" expr:"int($) >= 0"`

	// Timeout is the handling budget: one context deadline bounding how long the
	// inbound call may take. 0 means the Guard imposes none.
	Timeout time.Duration `value:"${timeout:=0}" expr:"int($) >= 0"`
}

// IsZero reports whether p configures no protection at all.
func (p Policy) IsZero() bool {
	return p.RateLimit == 0 && p.Burst == 0 && p.RateLimitMaxWait == 0 &&
		p.Algorithm == "" && p.Window == 0 &&
		p.RateLimitWarmUpDuration == 0 && p.WarmUpColdFactor == 0 &&
		p.MaxConcurrent == 0 && p.BulkheadQueueDuration == 0 &&
		p.Timeout == 0
}

// Settings is the inbound half of the governance document: one default policy
// plus per-instance overrides. This type IS the document — the control-plane
// admission root binds it and hands it back unchanged — so there is no
// separate binding twin.
type Settings struct {
	// Default governs every instance no Rule matches.
	Default Policy `value:"${:=}"`

	// Rules are the per-instance overrides, matched by exact Service. An empty
	// Service matches nothing (use Default for the catch-all).
	Rules []Rule `value:"${services:=}"`
}

// Validate is called by conf.Bind, so an invalid document is rejected at the
// source boundary rather than reaching the center.
func (s Settings) Validate() error {
	seen := make(map[string]struct{}, len(s.Rules))
	for i := range s.Rules {
		r := &s.Rules[i]
		if r.Service == "" {
			return errutil.Explain(nil, "admission: entry without a service")
		}
		if _, dup := seen[r.Service]; dup {
			return errutil.Explain(nil, "admission: duplicate entry for service %q", r.Service)
		}
		seen[r.Service] = struct{}{}
	}
	return nil
}

// Rule is one per-instance override. A matched Rule's Policy REPLACES the
// default whole rather than merging field-wise.
type Rule struct {
	// Service is the instance label this Rule matches, exact-compare.
	Service string `value:"${service:=}"`

	// Policy is what the matched instance runs under.
	Policy
}
