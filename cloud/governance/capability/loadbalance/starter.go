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
// The manager needs no strategy directory: names resolve against the package
// registry ([Register]), which every strategy file — and every package that
// registers one, a company's balancer say — has already populated by init. So
// there is no install step and no window in which a manager exists without its
// strategies, and no constructor parameter for a deployment to fill in.
//
// The bean owns its own teardown: close runs on [Manager.Close], which drops
// the chain registrations — the only references the manager keeps. gs destroys
// beans in reverse construction order, so the chains — built after the
// manager — are gone by then, which is the order the one-way registration
// requires.
func init() {
	gs.Provide(NewManager).Destroy((*Manager).Close).Caller(1)
}
