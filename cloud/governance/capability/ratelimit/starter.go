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

package ratelimit

import (
	"go-spring.org/spring/gs"
)

// The process-local provider is contributed as a [Provider] bean, and [Factory]
// collects it with every backend's provider (a Redis store, say) into the index
// Create looks up. The factory is the construction-time dependency both
// directions' managers take; what each provider answers is
// [Provider.Algorithms], and bean names carry no semantics.
//
// This file is only the container wiring — importing the package links it. The
// counting code itself stays dependency-free.
func init() {
	gs.Provide(NewLocalProvider)
	gs.Provide(NewFactory)
}
