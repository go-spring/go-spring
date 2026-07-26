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

package StarterConfigBus

// Config binds the config bus settings under the "spring.config.bus" prefix.
//
// The bus reuses an existing NATS connection (from starter-nats) as the
// transport. It carries only refresh *signals*, never configuration content —
// the source of truth stays the config center (or local files). Every instance
// subscribing to Subject re-runs the application property refresh when a signal
// arrives, giving Spring-Cloud-Bus-style "change once, refresh the whole fleet"
// semantics on top of whatever config sources are wired.
type Config struct {
	// Subject is the NATS subject that refresh events are published to and
	// subscribed from. All instances that share a subject form one bus.
	Subject string `value:"${subject:=spring.config.refresh}"`

	// WatchPrefixes optionally scopes which broadcasts this instance reacts to.
	// When empty the instance refreshes on every event. When set, a broadcast is
	// honored only if its prefix is empty (a full-fleet refresh) or overlaps one
	// of these prefixes, letting a heterogeneous fleet refresh selectively.
	// Comma-separated, e.g. "db,cache".
	WatchPrefixes string `value:"${watch-prefixes:=}"`
}
