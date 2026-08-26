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

// Package observe is a uniform trace+metric+log observer for client operations
// (cache, database, messaging). Each client starter attaches its instrumentation
// seam — a driver hook, a connection wrapper, a command monitor, a gorm callback
// — to one Observer, so every client emits the same three signals with the same
// vocabulary, behind one verbosity switch. That replaces the per-starter trace-only
// or span-helper-only instrumentation that drifted across the starter family.
//
// All three signals ride the OTel globals (TracerProvider / MeterProvider) that
// starter-otel installs. When starter-otel is absent those globals are no-ops, so
// trace and metric cost almost nothing and change no behavior; the access log
// (the third signal) always emits through the project log package, gated only by
// ObserveConfig.Level.
package observe

// ObserveConfig is the per-instance observability config, bound under
// spring.<client>.<name>.observability by each starter's own Config. It carries
// only the knobs that are meaningfully per-instance: the access-log level/detail
// and SkipOps (which suppresses span + metric + log together for chatty ops).
// Trace and metric themselves ride the global OTel pipeline that starter-otel
// installs, so they have no per-instance config here. Mirrors gin's
// AccessLogConfig.PayloadConfig.
type ObserveConfig struct {
	// Level controls access-log detail:
	//
	//   "off"      - no access log. Trace and metric still emit; only the log
	//                signal is silenced (e.g. for very high-volume clients).
	//   "brief"    - one record per operation carrying system, operation, status,
	//                duration, and error. The default: enough to spot slow/failing
	//                ops without pulling arguments.
	//   "detailed" - brief plus the operation argument (command name + key, sql
	//                statement, cache key, message topic, ...), bounded by
	//                MaxArgBytes so a large statement or payload can't flood the
	//                log. Use for troubleshooting.
	Level string `value:"${level:=brief}"`

	// MaxArgBytes caps how many bytes of the operation argument are captured in
	// "detailed" mode (per record). Defaults to 512, enough for a typical command
	// key or short statement without letting a runaway payload exhaust memory or
	// blow up the log line. Set higher only when you need fuller statements.
	MaxArgBytes int `value:"${maxArgBytes:=512}"`

	// SkipOps suppresses all three signals (span, metric, access log) for the
	// listed operations, so a chatty or noisy operation — a Redis PING, a health
	// probe, a hot cache key — does not flood the backends. An entry matches the
	// operation name passed to Observer.Start (e.g. "PING", "ping", "health").
	SkipOps []string `value:"${skipOps:=}"`
}

// level constants — compared as strings so a starter can bind Level straight from
// config without an enum parse step.
const (
	levelOff      = "off"
	levelBrief    = "brief"
	levelDetailed = "detailed"

	// DefaultBrief is the default Level value ("brief"), exported so starters
	// that build an ObserveConfig programmatically (rather than binding from config)
	// reference the same default.
	DefaultBrief = levelBrief
)

// Enabled reports whether the access log emits at the configured level. An
// unset Level ("") behaves as the "brief" default, so only "off" silences the
// log signal. Exported so adapters outside this package (e.g. the resilience
// and lock bridges) gate their logging through the same semantics instead of
// comparing the Level literal themselves.
func (c ObserveConfig) Enabled() bool { return c.Level != levelOff }

// Detailed reports whether the operation argument is captured into the log.
// Exported for the same reason as Enabled.
func (c ObserveConfig) Detailed() bool { return c.Level == levelDetailed }

// maxArg returns the argument capture bound, defaulting to 512 when unset.
func (c ObserveConfig) maxArg() int {
	if c.MaxArgBytes > 0 {
		return c.MaxArgBytes
	}
	return 512
}
