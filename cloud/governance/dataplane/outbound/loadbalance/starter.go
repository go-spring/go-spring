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

package loadbalance

import (
	"go-spring.org/spring/gs"
)

// Package registration for the load-balancing capability.
//
// # The manager
//
// NewManager is registered BY this package, so a client that injects a
// *loadbalance.Manager has already imported it. The dependency is required: an
// absent bean fails startup rather than silently degrading into an unmanaged
// client.
//
// # Strategies
//
// A [Provider] exports one or more strategy names; this package registers the
// built-in provider, and any deployment that adds a strategy contributes its own
// Provider bean. [Factory] collects all of them into the name index and is the
// manager's required dependency.
//
// # Teardown
//
// The manager holds no goroutines and no connections; its live state is the
// chain registration table, so [Manager.Close] is the destroy hook that drops
// it. gs destroys beans in reverse construction order, and chains are built
// after the manager, so they are already gone by the time Close runs.
func init() {
	gs.Provide(NewBuiltinProvider)
	gs.Provide(NewFactory)
	// The OTel-backed [Observer] is the process default; an application that
	// contributes its own Observer bean takes precedence (OnMissingBean), so the
	// layer's observability can be adapted without touching the manager.
	gs.Provide(NewOTelObserver).Condition(gs.OnMissingBean[Observer]()).Caller(1)
	// The manager's static configuration is bound once, under its own prefix —
	// never from the pushed selection document. The selection root consumes
	// spring.governance.client.loadbalance; these are operational switches.
	gs.Provide(NewManager,
		gs.IndexArg(2, gs.TagArg("${spring.governance.loadbalance}")),
	).Destroy((*Manager).Close)
}
