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

// Config is the luohua baseline's process-wide knobs, bound from ${spring.luohua.*}.
// It carries only what is process-wide — the master switch and the two value
// re-basing capabilities (propagate/observability). The keyed bean capabilities
// (identity/i18n/redis/lock) each bind their OWN config from their
// ${spring.luohua.<cap>} prefix in their module and are armed independently; they
// are deliberately NOT nested here, because binding a required field (identity's
// secret) from the process-wide bind would force every capability to configure
// identity even when unused.
//
// Arming: this module is gated on the "spring.luohua" property *prefix*, so any
// spring.luohua.* key arms it; Enabled (default true) is the master switch to
// silence the whole baseline while leaving the config in place.
type Config struct {
	// Enabled is the master switch. Set false to import the starter but apply
	// none of the luohua baseline.
	Enabled bool `value:"${enabled:=true}"`

	// Propagate re-bases go-spring's wire vocabulary (which headers/metadata
	// ride the fleet and which header marks synthetic load) onto luohua's.
	Propagate PropagateConfig `value:"${propagate:=}"`

	// Observability sets the per-log/span fields luohua expects on context.
	Observability ObservabilityConfig `value:"${observability:=}"`
}

// RedisConfig configures luohua's redigo pool-assembly driver under
// ${spring.luohua.redis}.
type RedisConfig struct {
	// Tag is a luohua marker applied to every pool this driver assembles, e.g.
	// stamped on each redis command so fleet traffic is attributable to luohua.
	Tag string `value:"${tag:=luohua}"`
}

// IdentityConfig configures the luohua security.TokenValidator under
// ${spring.luohua.identity}.
type IdentityConfig struct {
	// Secret is the shared HMAC secret used to sign and verify luohua tokens.
	// Required: providing it is what arms the validator bean.
	Secret string `value:"${secret}" expr:"$ != ''"`

	// Issuer names the token issuer (default "luohua").
	Issuer string `value:"${issuer:=luohua}"`
}

// I18nConfig configures luohua's error catalog under ${spring.luohua.i18n}.
type I18nConfig struct {
	// DefaultLocale is the catalog fallback locale (default "zh").
	DefaultLocale string `value:"${default-locale:=zh}"`
}

// PropagateConfig is the luohua wire/propagation baseline under
// ${spring.luohua.propagate}. go-spring standard components each expose a seam
// here — a company adopts luohua's convention in one place and every transport
// follows (see the per-capability notes).
type PropagateConfig struct {
	// LoadTestHeader overrides the traffic.HeaderLoadTest marker so inbound and
	// outbound load-test detection uses luohua's own header. Empty keeps the
	// go-spring canonical "X-LoadTest".
	LoadTestHeader string `value:"${load-test-header:=}"`

	// Headers lists the luohua business headers a named-header propagator
	// carries across every hop (e.g. X-Tenant, X-User). They ride the OTel
	// global propagator, so httpx / gin / echo / grpc all honour them.
	Headers []string `value:"${headers:=}"`
}

// ObservabilityConfig sets which context fields luohua surfaces on every log
// line, under ${spring.luohua.observability}.
type ObservabilityConfig struct {
	// Fields are the context keys luohua reads into each log event (e.g.
	// tenant, user). An empty list keeps go-spring's defaults (trace_id only).
	Fields []string `value:"${fields:=}"`
}
