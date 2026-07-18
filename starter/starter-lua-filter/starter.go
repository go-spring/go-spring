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

package StarterLuaFilter

import (
	"go-spring.org/spring/gs"
)

func init() {
	// Register multiple Lua filters as a group, one per entry under
	// "${spring.lua.filter}". Each bean is named after its config sub-key
	// (e.g. spring.lua.filter.guard -> bean "guard"), so callers select a
	// filter with gs.TagArg("guard") when wiring their *gs.HttpServeMux.
	//
	// The Lua VM has no background goroutine, so no destroy callback is needed.
	gs.Group("${spring.lua.filter}", newFilter, nil)
}
