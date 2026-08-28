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

/*
Package log is a high-performance and extensible structured logging library for Go. Logs are
classified by registered tags, configured through a flat property map, and rendered by pluggable
Logger / Layout / Appender plugins.

# Usage

Register tags during package init, then log through them. Code never holds a Logger value: the
framework binds each tag to the most specific configured logger and rebinds atomically on
[RefreshConfig].

	var TagRequestIn = log.RegisterRPCTag("http", "in")

	log.Infof(ctx, TagRequestIn, "hello %s", name)

	log.Info(ctx, TagRequestIn,
		log.String("user_id", "10001"),
		log.Msg("login succeeded"),
	)

Trace and Debug take a lazy field builder so level-disabled call sites allocate nothing:

	log.Debug(ctx, TagRequestIn, func() []log.Field {
		return []log.Field{log.String("user_id", "10001")}
	})

Register a custom tag only when its logs need to be tuned independently of the main log
(access logs, retry/circuit-breaker events, background relays, third-party framework bridges);
everything else should use the default tags [TagAppDef] / [TagBizDef]. An unconfigured tag falls
back to the default logger — that is the baseline, not a degraded mode. See the package README
for the full guidance.

# Configuration

Logger topology is loaded from a flat property map; reading and parsing the configuration file
is the caller's job (e.g. via go-spring.org/stdlib/flatten). See the package README for the
configuration reference, built-in plugin list, and extension guide:
https://github.com/go-spring/log
*/
package log
