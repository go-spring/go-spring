/*
 * Copyright 2012-2019 the original author or authors.
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

package slice_test

import (
	"testing"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/container/slice"
)

func TestSlice_Append(t *testing.T) {

	// 共享 empty 切片不会影响 Append 结果。
	t.Run("empty", func(t *testing.T) {
		s1 := slice.New()
		s1.Append(3)
		s2 := slice.New()
		s2.Append(4)
		assert.NotEqual(t, s1, s2)
	})

	// 不能使用 slice.slice 的值类型
	t.Run("value", func(t *testing.T) {
		s1 := *slice.New()
		s1.Append(3)
		s2 := s1
		s2.Append(4)
		assert.NotEqual(t, s1, s2)
	})

	// 必须使用 slice.slice 的引用类型
	t.Run("ref", func(t *testing.T) {
		s1 := slice.New()
		s1.Append(3)
		s2 := s1
		s2.Append(4)
		assert.Equal(t, s1, s2)
	})
}
