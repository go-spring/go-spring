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
	StarterCache "go-spring.org/starter-cache"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register the luohua cache driver under one company-wide name. Any service
	// that wants the standard luohua in-process cache selects it by config —
	//
	//	spring.cache.<name>.driver = luohua:<beanID>
	//
	// — keeping the whole company on one driver spelling while the backend stays
	// swappable behind the same cloud/cache.ByteCache contract (M1 seam).
	StarterCache.RegisterDriver("luohua", func(beanID string) gs.ModuleFunc {
		return func(r gs.BeanProvider, _ flatten.Storage) error {
			r.Provide(func() *cache.Cache { return NewLuohuaCache() }).Name(beanID)
			return nil
		}
	})
}
