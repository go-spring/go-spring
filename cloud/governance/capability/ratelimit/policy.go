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
// Which fields an algorithm reads is the applicability matrix (see each
// algorithm's file); a field an algorithm does not read is ignored, never an
// error:
//
//	             Limiter  RateLimit  Window  Burst  MaxWait  WarmUp  ColdFactor
//	token-bucket    bind       ●        -       ●       ●        ●         ●
//	sliding-window  bind       ●        ●       -       -        -         -
//	fixed-window    bind       ●        ●       -       -        -         -
//
// "bind" marks the one binding-time-only knob (see [Policy.Limiter]); "-"
// means ignored by that algorithm. On top of the matrix: a zero RateLimit
// disables the stage outright, so every other field is moot.
//
// Every field is zero-value-safe, and a zero policy disables the stage.
type Policy struct {
	// Limiter names the algorithm that counts this stage: the bean name of a
	// [Provider] ("token-bucket", "sliding-window", "redis:token-bucket", ...).
	// The configuration binding defaults it to "token-bucket" when a document
	// omits it; a programmatic caller must name it. An unregistered algorithm
	// is rejected when the document is applied.
	//
	// It is the one field that is not a per-call knob: the algorithm is bound
	// when the [Factory] creates the counter, so an Allow implementation must
	// ignore it.
	Limiter string `value:"${limiter:=token-bucket}"`

	// RateLimit is the sustained budget in units per second. 0 disables the
	// stage.
	RateLimit float64 `value:"${rate-limit:=0}" expr:"$ >= 0"`

	// Window is the rolling interval the window algorithms count over; 0 means
	// one second.
	Window time.Duration `value:"${window:=0}" expr:"int($) >= 0"`

	// Burst is the token bucket's stockpile capacity: free permits that sit on
	// top of the sustained pace (Guava semantics — a charge never waits for its
	// own cost). The stockpile starts empty and regrows only while the bucket
	// is idle. Unset means about one second's worth of RateLimit (at least 1).
	// Warm-up derives its own capacity, so Burst is also ignored while WarmUp
	// is set.
	Burst int `value:"${burst:=0}" expr:"$ >= 0"`

	// MaxWait is how long a charge may wait for its turn at the head of the
	// bucket's pipeline before being refused (queueing / traffic shaping).
	// 0 (the default) admits only when the pipeline is free; a positive value
	// turns queued calls into bounded waits, smoothing bursts instead of
	// failing them. The wait also ends when the caller's ctx is done. A shared
	// store that cannot queue ignores it.
	MaxWait time.Duration `value:"${rate-limit-max-wait:=0}" expr:"int($) >= 0"`

	// WarmUp is the cold-start protection period, in Guava's stored-permit
	// sense: the bucket starts full and cold, and while traffic keeps its
	// stockpile above the warm threshold, permits cost up to ColdFactor times
	// their steady price — so a freshly restarted fleet does not hit a cold
	// downstream at full quota, and an idle one that has cooled down does not
	// either. The warm-up derives its own stockpile capacity from the period
	// and RateLimit (Burst is ignored while WarmUp is set). 0 disables
	// warm-up.
	WarmUp time.Duration `value:"${rate-limit-warm-up-duration:=0}" expr:"int($) >= 0"`

	// ColdFactor shapes the ramp-in: the coldest permit costs ColdFactor times
	// its steady price. 0 means the backend's default (3). Meaningful only
	// while WarmUp is set.
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
