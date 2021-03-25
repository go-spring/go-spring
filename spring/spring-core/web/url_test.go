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

package web_test

import (
	"testing"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/web"
)

func TestToPathStyle(t *testing.T) {

	t.Run("/:a", func(t *testing.T) {
		newPath, wildCardName := web.ToPathStyle("/:a", web.EchoPathStyle)
		assert.Equal(t, newPath, "/:a")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/:a", web.GinPathStyle)
		assert.Equal(t, newPath, "/:a")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/:a", web.JavaPathStyle)
		assert.Equal(t, newPath, "/{a}")
		assert.Equal(t, wildCardName, "")
	})

	t.Run("/{a}", func(t *testing.T) {
		newPath, wildCardName := web.ToPathStyle("/{a}", web.EchoPathStyle)
		assert.Equal(t, newPath, "/:a")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/{a}", web.GinPathStyle)
		assert.Equal(t, newPath, "/:a")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/{a}", web.JavaPathStyle)
		assert.Equal(t, newPath, "/{a}")
		assert.Equal(t, wildCardName, "")
	})

	t.Run("/:a/b/:c", func(t *testing.T) {
		newPath, wildCardName := web.ToPathStyle("/:a/b/:c", web.EchoPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/:a/b/:c", web.GinPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/:a/b/:c", web.JavaPathStyle)
		assert.Equal(t, newPath, "/{a}/b/{c}")
		assert.Equal(t, wildCardName, "")
	})

	t.Run("/{a}/b/{c}", func(t *testing.T) {
		newPath, wildCardName := web.ToPathStyle("/{a}/b/{c}", web.EchoPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/{a}/b/{c}", web.GinPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/{a}/b/{c}", web.JavaPathStyle)
		assert.Equal(t, newPath, "/{a}/b/{c}")
		assert.Equal(t, wildCardName, "")
	})

	t.Run("/:a/b/:c/*", func(t *testing.T) {
		newPath, wildCardName := web.ToPathStyle("/:a/b/:c/*", web.EchoPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/:a/b/:c/*", web.GinPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*@_@")
		assert.Equal(t, wildCardName, "@_@")
		newPath, wildCardName = web.ToPathStyle("/:a/b/:c/*", web.JavaPathStyle)
		assert.Equal(t, newPath, "/{a}/b/{c}/{*}")
		assert.Equal(t, wildCardName, "")
	})

	t.Run("/{a}/b/{c}/{*}", func(t *testing.T) {
		newPath, wildCardName := web.ToPathStyle("/{a}/b/{c}/{*}", web.EchoPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*")
		assert.Equal(t, wildCardName, "")
		newPath, wildCardName = web.ToPathStyle("/{a}/b/{c}/{*}", web.GinPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*@_@")
		assert.Equal(t, wildCardName, "@_@")
		newPath, wildCardName = web.ToPathStyle("/{a}/b/{c}/{*}", web.JavaPathStyle)
		assert.Equal(t, newPath, "/{a}/b/{c}/{*}")
		assert.Equal(t, wildCardName, "")
	})

	t.Run("/:a/b/:c/*e", func(t *testing.T) {
		newPath, wildCardName := web.ToPathStyle("/:a/b/:c/*e", web.EchoPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*")
		assert.Equal(t, wildCardName, "e")
		newPath, wildCardName = web.ToPathStyle("/:a/b/:c/*e", web.GinPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*e")
		assert.Equal(t, wildCardName, "e")
		newPath, wildCardName = web.ToPathStyle("/:a/b/:c/*e", web.JavaPathStyle)
		assert.Equal(t, newPath, "/{a}/b/{c}/{*:e}")
		assert.Equal(t, wildCardName, "e")
	})

	t.Run("/{a}/b/{c}/{*:e}", func(t *testing.T) {
		newPath, wildCardName := web.ToPathStyle("/{a}/b/{c}/{*:e}", web.EchoPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*")
		assert.Equal(t, wildCardName, "e")
		newPath, wildCardName = web.ToPathStyle("/{a}/b/{c}/{*:e}", web.GinPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*e")
		assert.Equal(t, wildCardName, "e")
		newPath, wildCardName = web.ToPathStyle("/{a}/b/{c}/{*:e}", web.JavaPathStyle)
		assert.Equal(t, newPath, "/{a}/b/{c}/{*:e}")
		assert.Equal(t, wildCardName, "e")
	})

	t.Run("/{a}/b/{c}/{e:*}", func(t *testing.T) {
		newPath, wildCardName := web.ToPathStyle("/{a}/b/{c}/{e:*}", web.EchoPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*")
		assert.Equal(t, wildCardName, "e")
		newPath, wildCardName = web.ToPathStyle("/{a}/b/{c}/{e:*}", web.GinPathStyle)
		assert.Equal(t, newPath, "/:a/b/:c/*e")
		assert.Equal(t, wildCardName, "e")
		newPath, wildCardName = web.ToPathStyle("/{a}/b/{c}/{e:*}", web.JavaPathStyle)
		assert.Equal(t, newPath, "/{a}/b/{c}/{*:e}")
		assert.Equal(t, wildCardName, "e")
	})
}
