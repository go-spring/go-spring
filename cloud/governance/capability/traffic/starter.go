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

// The process's default [Propagator] bean, registered by the package that owns
// the convention so every component can inject one type without each starter
// re-declaring it. This registration is the package's only global state.
//
// [gs.OnMissingBean] makes it a default rather than an override: an application
// that re-bases the convention — [DefaultBinding] with the fields that differ
// overwritten, fed to [NewDefaultPropagator], or a custom [Propagator]
// implementation — registers that as a bean of the same type, and this one
// steps aside. The container then holds exactly one propagator per process,
// whichever of the two it is.
//
// [gs.Caller] records this line as the bean's origin, so a container
// diagnostic names the package that declared the default rather than an
// internal gs frame.
func init() {
	gs.Provide(func() Propagator {
		return NewDefaultPropagator(DefaultBinding())
	}).Condition(gs.OnMissingBean[Propagator]()).Caller(1)
}
