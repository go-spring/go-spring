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

package luohua

import (
	"go-spring.org/cloud/cache"
	"go-spring.org/spring/gs"
)

func init() {
	// Expose the standard luohua in-process cache as a *cache.Cache bean under
	// one company-wide name — autowire it as `cache.Cache` with tag "luohua".
	// Un-injected, the bean never instantiates, so no config gate is needed.
	gs.Provide(func() *cache.Cache { return NewLuohuaCache() }).Name("luohua")
}
