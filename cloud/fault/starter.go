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

// The injector is registered here, by the package that owns it: a client that
// injects *fault.Injector has already imported this package, and its
// constructor parameter is REQUIRED — an absent bean fails startup rather than
// degrading silently. With no rule armed it is a transparent pass-through, so
// having it always present costs nothing. The propagator bean is injected by
// type; cloud/traffic's starter guarantees one exists.
func init() {
	gs.Provide(func(prop traffic.Propagator) *Injector {
		return NewInjector(Configs{Client: Config{}}, prop)
	}).Caller(1)
}
