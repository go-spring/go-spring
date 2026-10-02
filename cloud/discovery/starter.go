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

// The registration server is registered here, by the package that owns it, so
// linking this package is what puts it in the container: importing any backend
// starter imports this package transitively, and the bean exists exactly once
// per process by Go's package-init semantics — no coordination between
// backends is needed.
func init() {
	// Activated only when the registration intent signal is set: a
	// ${spring.discovery.service-name} means this process publishes itself.
	// Pure consumers leave it unset and nothing registers anywhere. The
	// Registries field is the container's slice collection of every
	// discovery.Registry bean the backend starters derived from their
	// configured blocks.
	gs.Provide(NewServer).
		Name("discoveryServer").
		Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.discovery.service-name"))
}
