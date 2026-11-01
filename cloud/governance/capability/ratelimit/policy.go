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

package ratelimit

import (
	"time"
)

// Policy is the rate-limit stage's vocabulary and the only type describing its
// knobs: a rule writes it, a direction document embeds it, and a [Limiter]
// takes it on every call. One type serves both roles because the runtime view
// and the configuration view are the same knobs — splitting them would only add
// a field-by-field mapping to keep in sync.
//
// Every field is zero-value-safe, and a zero policy disables the stage.
type Policy struct {
	// RateLimit is the sustained budget in units per second. 0 disables the
	// stage.
	RateLimit float64 `value:"${rate-limit:=0}" expr:"$ >= 0"`

	// Burst is the largest charge the token bucket can hold: it starts full and
	// refills toward Burst. Unset means about one second's worth of RateLimit
	// (at least 1). Ignored by the window algorithms and when RateLimit is 0.
	Burst int `value:"${burst:=0}" expr:"$ >= 0"`

	// MaxWait is how long an over-limit charge may WAIT for a unit before being
	// refused (queueing / traffic shaping). 0 (the default) refuses
	// immediately; a positive value turns excess calls into bounded waits,
	// smoothing bursts instead of failing them. The wait also ends when the
	// caller's ctx is done. Ignored when RateLimit is 0, and by shared stores
	// that cannot queue.
	MaxWait time.Duration `value:"${rate-limit-max-wait:=0}" expr:"int($) >= 0"`

	// Limiter names the algorithm that counts this stage: the bean name of a
	// [Provider] ("token-bucket", "sliding-window", "redis:token-bucket", ...).
	// The configuration binding defaults it to "token-bucket" when a document
	// omits it; a programmatic caller must name it. Ignored when RateLimit is 0.
	// An unregistered algorithm is rejected when the document is applied.
	//
	// It is the one field that is not a per-call knob: the algorithm is bound
	// when the [Factory] creates the counter, so an Allow implementation must
	// ignore it.
	Limiter string `value:"${limiter:=token-bucket}"`

	// Window is the rolling interval [SlidingWindowAlgorithm] counts over; 0 means
	// one second. Ignored by the token bucket and when RateLimit is 0.
	Window time.Duration `value:"${window:=0}" expr:"int($) >= 0"`

	// WarmUp is the cold-start ramp-in period, instead of admitting the full
	// RateLimit the instant the process starts — a freshly restarted fleet must
	// not hit a cold downstream at full quota. A backend maps it onto its own
	// cold-start mechanism (the default one grows a token bucket's capacity
	// linearly over the duration). 0 disables warm-up. Ignored by the window
	// algorithms and when RateLimit is 0.
	WarmUp time.Duration `value:"${rate-limit-warm-up-duration:=0}" expr:"int($) >= 0"`

	// ColdFactor shapes the ramp-in: admission starts at roughly
	// RateLimit/ColdFactor and rises to full by the end of [Policy.WarmUp]. 0
	// means the backend's default (3). Ignored when WarmUp is 0.
	ColdFactor int `value:"${warm-up-cold-factor:=0}" expr:"$ >= 0"`
}

// IsZero reports whether this rate-limit policy configures nothing.
func (p Policy) IsZero() bool {
	return p.RateLimit == 0 && p.Burst == 0 && p.MaxWait == 0 &&
		p.Limiter == "" && p.Window == 0 && p.WarmUp == 0 && p.ColdFactor == 0
}

// burstFor is the capacity a token bucket starts with and refills toward; a
// missing burst means about one second's worth of the rate (at least 1).
func burstFor(p Policy) int {
	if p.Burst > 0 {
		return p.Burst
	}
	return max(int(p.RateLimit), 1)
}

// windowLimit reads the rolling window a policy asks for — one second when it
// names none — and the per-window cap that window implies (at least 1).
func windowLimit(p Policy) (time.Duration, float64) {
	window := p.Window
	if window <= 0 {
		window = time.Second
	}
	return window, max(p.RateLimit*window.Seconds(), 1)
}
