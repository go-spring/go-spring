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
	"time"
)

// PolicyConfig binds the resilience policy knobs from go-spring ${...} value
// tags and yields a [Policy]. It is the policy-only half of [Config]: no
// Enabled/Driver, because those are deployment-level switches. The governance
// center embeds it per Default/Rule (govern.default.*, govern.rules[n].*), so
// the on/off switch and backend selection stay process-wide at the top of
// ${govern} (govern.enabled, govern.driver) instead of being re-bindable per
// resource.
//
// Every field uses the same kebab-case key style and the same defaults as the
// historical per-starter configs, so existing ${resilience.*} properties keep
// working unchanged; the newer knobs (max-concurrent, backoff, max-duration,
// breaker-strategy) all default to off.
type PolicyConfig struct {
	// --- rate limit ---

	// RateLimit caps sustained throughput in operations per second (0 disables).
	RateLimit float64 `value:"${rate-limit:=0}"`

	// Burst is the momentary allowance above RateLimit (0 = driver default).
	Burst int `value:"${burst:=0}"`

	// --- circuit breaker ---

	// ErrorThreshold is the failure count that trips the breaker under the
	// "consecutive" strategy (0 disables that strategy).
	ErrorThreshold int `value:"${error-threshold:=0}"`

	// OpenDuration is how long the breaker stays open before a trial attempt.
	OpenDuration time.Duration `value:"${open-duration:=0}"`

	// BreakerStrategy selects how the breaker counts ("consecutive" default, or
	// "error-rate"). Empty means "consecutive".
	BreakerStrategy BreakerStrategy `value:"${breaker-strategy:=}"`

	// ErrorRateThreshold is the failure ratio in (0,1] that trips an
	// "error-rate" breaker (0 disables that strategy).
	ErrorRateThreshold float64 `value:"${error-rate-threshold:=0}"`

	// MinRequests is the minimum sample in the window before an "error-rate"
	// breaker may trip (default 1).
	MinRequests int `value:"${min-requests:=0}"`

	// BreakerWindow is the rolling interval the "error-rate" strategy counts
	// over (default 1s).
	BreakerWindow time.Duration `value:"${breaker-window:=0}"`

	// --- bulkhead ---

	// MaxConcurrent caps in-flight operations against a resource (0 disables).
	// Excess calls are rejected with [ErrBulkheadFull] rather than queued.
	MaxConcurrent int `value:"${max-concurrent:=0}"`

	// --- retry ---

	// MaxRetries is the number of extra attempts after the first failure.
	MaxRetries int `value:"${max-retries:=0}"`

	// InitialInterval is the backoff before the first retry (0 = no backoff).
	InitialInterval time.Duration `value:"${initial-interval:=0}"`

	// Multiplier is the exponential growth factor (0/1 = constant interval).
	Multiplier float64 `value:"${multiplier:=0}"`

	// MaxInterval caps the grown backoff (0 = unbounded).
	MaxInterval time.Duration `value:"${max-interval:=0}"`

	// RandomizationFactor is the jitter fraction in [0,1) (0 = no jitter).
	RandomizationFactor float64 `value:"${randomization-factor:=0}"`

	// --- timeouts ---

	// AttemptTimeout bounds each individual attempt (0 = no per-attempt bound).
	AttemptTimeout time.Duration `value:"${attempt-timeout:=0}"`

	// MaxDuration caps the wall time of the whole call across all retries
	// (0 = no total cap).
	MaxDuration time.Duration `value:"${max-duration:=0}"`

	// RetryPredicateFn, when non-nil, is set on the produced [Policy] as
	// [Policy.RetryPredicate]. It is NOT bound from value tags (funcs cannot
	// be): a client adapter assigns it in code after binding Config, e.g. a
	// write-suppressing predicate for non-idempotent commands. Adapters that
	// need classification should prefer returning a [Retryable]-implementing
	// error from inside fn over setting this.
	RetryPredicateFn func(error) bool
}

// Config is what a client starter embeds: the policy knobs ([PolicyConfig])
// plus the per-starter Enabled switch and Driver selection, all bound under the
// starter's own prefix (e.g. ${resilience.enabled}, ${resilience.driver}). It
// composes PolicyConfig rather than repeating it, so a starter that embeds
// Config keeps exactly its old binding surface, while governance — which owns
// Enabled/Driver centrally — embeds only the PolicyConfig half.
type Config struct {
	PolicyConfig

	// Enabled turns resilience on. When false the host client is unchanged.
	Enabled bool `value:"${enabled:=false}"`

	// Driver names the registered resilience backend ("default" or "sentinel").
	Driver string `value:"${driver:=default}"`
}

// Policy maps the bound policy config onto the driver-neutral [Policy]. It is
// the single translation point every client starter uses, replacing the
// per-starter policy() copies. Defined on PolicyConfig (and thus reachable as
// Config.Policy too via embedding).
func (c PolicyConfig) Policy() Policy {
	return Policy{
		RateLimit:           c.RateLimit,
		Burst:               c.Burst,
		ErrorThreshold:      c.ErrorThreshold,
		OpenDuration:        c.OpenDuration,
		BreakerStrategy:     c.BreakerStrategy,
		ErrorRateThreshold:  c.ErrorRateThreshold,
		MinRequests:         c.MinRequests,
		BreakerWindow:       c.BreakerWindow,
		MaxConcurrent:       c.MaxConcurrent,
		MaxRetries:          c.MaxRetries,
		RetryPredicate:      c.RetryPredicateFn,
		InitialInterval:     c.InitialInterval,
		Multiplier:          c.Multiplier,
		MaxInterval:         c.MaxInterval,
		RandomizationFactor: c.RandomizationFactor,
		Timeout:             c.AttemptTimeout,
		MaxDuration:         c.MaxDuration,
	}
}

// ResourceLabel joins prefix with the first non-empty name in names, falling
// back to prefix alone when none is set. Client starters call it to standardize
// the resilience resource key each driver scopes limiter and breaker state by:
//
//	resilience.ResourceLabel("redis", c.ServiceName, c.MasterName, c.Addr)
//
// The client-specific fallback chain (only the client knows its own address
// fields) stays in the client; this helper just standardizes the join.
func ResourceLabel(prefix string, names ...string) string {
	for _, n := range names {
		if n != "" {
			return prefix + ":" + n
		}
	}
	return prefix
}
