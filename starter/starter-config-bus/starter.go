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

// Package StarterConfigBus adds a configuration refresh bus on top of an
// existing NATS connection (starter-nats). Blank-importing this package
// registers a ConfigBus bean that subscribes to a refresh subject and re-runs
// the application-wide property refresh whenever a signal arrives, so a change
// broadcast once refreshes every instance in the fleet.
//
// It complements the remote config-center starters (starter-config-{nacos,
// etcd,consul}): those already refresh a single instance from their own watch,
// while the bus covers cross-instance broadcast and refreshes triggered from
// outside the config center. The bus carries refresh *signals* only — never
// configuration content, which stays with the config center or local files.
//
// Configure the transport by pointing spring.config.bus.nats-instance at a
// connection defined under spring.nats.* (default instance name
// "config-bus").
package StarterConfigBus

import (
	"go-spring.org/spring/gs"
)

func init() {
	// Register the bus as a named root object so it is always created (wiring
	// the refresh listener regardless of whether anything else depends on it)
	// without its Rooter export colliding with the application's own default
	// Rooter. Inject it elsewhere via autowire:"configBus".
	gs.Provide(&ConfigBus{}).
		Name("configBus").
		Init((*ConfigBus).subscribe).
		Destroy((*ConfigBus).close).
		Export(gs.As[gs.Rooter]())
}
