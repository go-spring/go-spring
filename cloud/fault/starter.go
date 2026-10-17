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

package fault

import (
	"go-spring.org/cloud/traffic"
	"go-spring.org/spring/gs"
)

// The two authorities are registered here, by the package that owns them: a
// starter that injects *ClientManager or *ServerManager has already imported this
// package, and its constructor parameter is REQUIRED — an absent bean fails
// startup rather than degrading silently. With no rule armed each is a
// transparent pass-through, so having it always present costs nothing.
//
// They are two beans, not one container bean: a client starter must not be able
// to reach the inbound authority, nor a server starter the outbound one. The
// governance center injects both because it owns the one document that drives
// them, but each direction is still an independent object with its own API — the
// center does not aggregate them into a third type.
//
// The propagator bean is injected by type; cloud/traffic's starter guarantees one
// exists.
func init() {
	gs.Provide(func(prop traffic.Propagator) *ClientManager {
		return NewClientManager(prop)
	}).Caller(1)

	gs.Provide(func(prop traffic.Propagator) *ServerManager {
		return NewServerManager(prop)
	}).Caller(1)
}
