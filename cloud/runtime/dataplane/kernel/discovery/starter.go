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

package discovery

import (
	"go-spring.org/spring/gs"
)

// The read-side directory is registered here, by the package that owns it, so
// linking this package is what puts it in the container: importing any backend
// starter imports this package transitively, and the bean exists exactly once
// per process by Go's package-init semantics — no coordination between
// backends is needed. The write-side registration server is registered by
// cloud/runtime/controlplane/registration, the package that owns that half.
func init() {
	// The directory of every discovery.Discovery bean in the container, keyed by
	// bean name (the label a client cites). It is the authority the runtime
	// composition root holds and hands out, so a client reaches discovery
	// through the runtime like every other authority. The map is optional (nil
	// when the app declares no backend bean), and a nil map is an empty
	// directory — every lookup misses.
	gs.Provide(func(backends map[string]Discovery) *Manager {
		return NewManager(backends)
	}, gs.IndexArg(0, gs.TagArg("?"))).Caller(1)
}
