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

// The manager is registered here, by the package that owns it: a client that
// injects *loadbalance.Manager has already imported this package, and its
// constructor parameter is REQUIRED — an absent bean fails startup rather than
// degrading silently.
//
// Strategies are [Provider] beans: this package contributes the built-in provider,
// and any package that supplies a strategy contributes its own. [Factory]
// collects them and is the manager's required dependency.
//
// The bean owns its own teardown: close runs on [Manager.Close], which drops
// the chain registrations — the only references the manager keeps. gs destroys
// beans in reverse construction order, so the chains — built after the
// manager — are gone by then.
func init() {
	gs.Provide(NewBuiltinProvider)
	gs.Provide(NewFactory)
	gs.Provide(NewManager).Destroy((*Manager).Close)
}
