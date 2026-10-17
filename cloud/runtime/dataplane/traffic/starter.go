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

package traffic

import (
	"go-spring.org/spring/gs"
)

// The default propagator bean is registered here, by the package that owns the
// convention: one process has one propagator, and a component that injects
// [Propagator] gets go-spring's [DefaultBinding] unless the application
// registers its own bean first — OnMissingBean makes the default step aside the
// moment a re-based convention (a custom binding or a custom implementation)
// enters the container. Components no longer normalize a nil propagator at each
// leaf; a nil is now only possible for a standalone caller outside the
// container, whom leaf constructors still serve with the same fallback.
func init() {
	gs.Provide(func() Propagator {
		return NewDefaultPropagator(DefaultBinding())
	}).Condition(gs.OnMissingBean[Propagator]()).Caller(1)
}
