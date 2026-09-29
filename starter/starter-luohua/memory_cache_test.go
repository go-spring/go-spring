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
	"testing"

	"go-spring.org/cloud/cache"
	"go-spring.org/spring/gs"
)

// TestLuohuaCacheBean verifies the cache seam: importing starter-luohua provides
// a *cache.Cache bean under the one company-wide name "luohua", autowirable by
// that name and backed by the standard luohua in-process cache.
func TestLuohuaCacheBean(t *testing.T) {
	gs.Web(false).RunTest(t, func(ts *struct {
		Cache *cache.Cache `autowire:"luohua"`
	}) {
		if ts.Cache == nil {
			t.Fatal("expected the luohua cache bean to be autowirable by name")
		}
	})
}
