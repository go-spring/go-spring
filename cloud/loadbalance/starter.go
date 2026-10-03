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
// The strategy directory is a constructor parameter: gs collects every
// [Factory] bean the container holds (a deployment contributes one with
// gs.Provide(...).Name("<strategy>").Export(gs.As[Factory]())) and hands the map
// over at construction, so there is no separate install step and no window in
// which a manager exists without its strategies. The tag is nullable because
// contributing none is the common case, and an empty collection is otherwise an
// injection error.
func init() {
	gs.Provide(func(factories map[string]Factory) (*Manager, error) {
		return NewManager(factories)
	}, gs.TagArg("?")).Caller(1)
}
