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
	"go-spring.org/spring/gs"
)

// The manager is registered here, by the package that owns it, so linking this
// package is what puts it in the container: a starter that injects
// *resilience.Manager has already imported it, and its constructor parameter is
// REQUIRED (`gs.IndexArg(n, gs.TagArg(""))`) — the bean being absent would fail
// startup, not degrade silently.
//
// This is the OUTBOUND authority only. Inbound requests run through
// go-spring.org/cloud/governance/dataplane/inbound/admission, whose manager is owned by the governance center.
//
// The manager is self-sufficient: its chain node reads the manager's current
// policy on every call, so it works with no center anywhere. A governance
// center, when one is linked, only pushes policy into it.
func init() {
	gs.Provide(NewManager).Destroy((*Manager).Close)
}
