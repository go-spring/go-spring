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

package internal_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/conf/internal"
)

func TestStorage(t *testing.T) {

	var s internal.Storage

	// 初始值是简单的 KV 值
	{
		s.Init()

		err := s.Set("a", "b")
		assert.Nil(t, err)
		assert.True(t, s.Has("a"))
		assert.Equal(t, s.Get("a"), "b")
		err = s.Set("a[0]", "x")
		assert.Error(t, err, "property \"a\" is a value but \"a\\[0]\" wants other type")
		err = s.Set("a.y", "x")
		assert.Error(t, err, "property \"a\" is a value but \"a\\.y\" wants other type")

		err = s.Set("a", "c")
		assert.Nil(t, err)
		assert.True(t, s.Has("a"))
		assert.Equal(t, s.Get("a"), "c")
		err = s.Set("a", "")
		assert.Nil(t, err)
		assert.True(t, s.Has("a"))
		assert.Equal(t, s.Get("a"), "")
		err = s.Set("a[0]", "x")
		assert.Error(t, err, "property \"a\" is a value but \"a\\[0]\" wants other type")
		err = s.Set("a.y", "x")
		assert.Error(t, err, "property \"a\" is a value but \"a\\.y\" wants other type")

		err = s.Set("a", "c")
		assert.Nil(t, err)
		assert.True(t, s.Has("a"))
		assert.Equal(t, s.Get("a"), "c")
		err = s.Set("a[0]", "x")
		assert.Error(t, err, "property \"a\" is a value but \"a\\[0]\" wants other type")
		err = s.Set("a.y", "x")
		assert.Error(t, err, "property \"a\" is a value but \"a\\.y\" wants other type")
	}

	// 初始值是嵌套的 KV 值
	{
		s.Init()

		err := s.Set("m.x", "y")
		assert.Nil(t, err)
		assert.True(t, s.Has("m"))
		assert.True(t, s.Has("m.x"))
		assert.Equal(t, s.Get("m.x"), "y")
		err = s.Set("m", "w")
		assert.Error(t, err, "property \"m\" is a map but \"m\" wants other type")
		err = s.Set("m[0]", "f")
		assert.Error(t, err, "property \"m\" is a map but \"m\\[0]\" wants other type")

		err = s.Set("m.x", "z")
		assert.Nil(t, err)
		assert.True(t, s.Has("m"))
		assert.True(t, s.Has("m.x"))
		assert.Equal(t, s.Get("m.x"), "z")
		err = s.Set("m", "")
		assert.Nil(t, err)
		assert.True(t, s.Has("m"))
		assert.False(t, s.Has("m.x"))
		err = s.Set("m", "w")
		assert.Error(t, err, "property \"m\" is a map but \"m\" wants other type")
		err = s.Set("m[0]", "f")
		assert.Error(t, err, "property \"m\" is a map but \"m\\[0]\" wants other type")

		err = s.Set("m.t", "q")
		assert.Nil(t, err)
		assert.True(t, s.Has("m"))
		assert.False(t, s.Has("m.x"))
		assert.True(t, s.Has("m.t"))
		assert.Equal(t, s.Get("m.x"), "")
		assert.Equal(t, s.Get("m.t"), "q")
		err = s.Set("m", "w")
		assert.Error(t, err, "property \"m\" is a map but \"m\" wants other type")
		err = s.Set("m[0]", "f")
		assert.Error(t, err, "property \"m\" is a map but \"m\\[0]\" wants other type")
	}

	// 初始值是数组 KV 值
	{
		s.Init()

		err := s.Set("s[0]", "p")
		assert.Nil(t, err)
		assert.True(t, s.Has("s"))
		assert.True(t, s.Has("s[0]"))
		assert.Equal(t, s.Get("s[0]"), "p")
		err = s.Set("s", "w")
		assert.Error(t, err, "property \"s\" is a list but \"s\" wants other type")
		err = s.Set("s.x", "f")
		assert.Error(t, err, "property \"s\" is a list but \"s\\.x\" wants other type")

		err = s.Set("s[0]", "q")
		assert.Nil(t, err)
		assert.True(t, s.Has("s"))
		assert.True(t, s.Has("s[0]"))
		assert.Equal(t, s.Get("s[0]"), "q")
		err = s.Set("s", "")
		assert.Nil(t, err)
		assert.True(t, s.Has("s"))
		assert.False(t, s.Has("s[0]"))
		err = s.Set("s", "w")
		assert.Error(t, err, "property \"s\" is a list but \"s\" wants other type")
		err = s.Set("s.x", "f")
		assert.Error(t, err, "property \"s\" is a list but \"s\\.x\" wants other type")

		err = s.Set("s[1]", "o")
		assert.Nil(t, err)
		assert.True(t, s.Has("s"))
		assert.False(t, s.Has("s[0]"))
		assert.True(t, s.Has("s[1]"))
		assert.Equal(t, s.Get("s[0]"), "")
		assert.Equal(t, s.Get("s[1]"), "o")
		err = s.Set("s", "w")
		assert.Error(t, err, "property \"s\" is a list but \"s\" wants other type")
		err = s.Set("s.x", "f")
		assert.Error(t, err, "property \"s\" is a list but \"s\\.x\" wants other type")
	}
}
