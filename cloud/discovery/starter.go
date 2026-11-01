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
// linking this package is what puts it in the container. Every client and
// backend starter imports this package, and Go's package-init semantics run
// this exactly once per process — the bean exists once and no starter has to
// coordinate. The write side (publishing this process to a naming service) is
// registered by cloud/governance/controlplane/registration, the package that owns
// that half.
func init() {
	// Collect every discovery.Discovery bean in the container into one Manager,
	// keyed by bean name — the name a client cites (e.g. "etcd.main"). The map
	// is nullable ("?"): an app that declares no backend bean gets a nil map,
	// which is a valid empty directory where every lookup misses. The runtime
	// composition root holds this Manager and hands it out as one of its
	// authorities, so a client reaches discovery through the runtime like every
	// other authority.
	gs.Provide(func(backends map[string]Discovery) *Manager {
		return NewManager(backends)
	}, gs.IndexArg(0, gs.TagArg("?"))).Caller(1)
}
