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

package SpringWeb_test

import (
	"testing"

	"github.com/go-spring/spring-web"
	"github.com/magiconair/properties/assert"
)

func TestToPathStyle(t *testing.T) {

	t.Run("/:a", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/:a", SpringWeb.EchoPathStyle)
		assert.Equal(t, "/:a", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a", SpringWeb.GinPathStyle)
		assert.Equal(t, "/:a", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a", SpringWeb.JavaPathStyle)
		assert.Equal(t, "/{a}", newPath)
		assert.Equal(t, "", wildCardName)
	})

	t.Run("/{a}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}", SpringWeb.EchoPathStyle)
		assert.Equal(t, "/:a", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}", SpringWeb.GinPathStyle)
		assert.Equal(t, "/:a", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}", SpringWeb.JavaPathStyle)
		assert.Equal(t, "/{a}", newPath)
		assert.Equal(t, "", wildCardName)
	})

	t.Run("/:a/b/:c", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/:a/b/:c", SpringWeb.EchoPathStyle)
		assert.Equal(t, "/:a/b/:c", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c", SpringWeb.GinPathStyle)
		assert.Equal(t, "/:a/b/:c", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c", SpringWeb.JavaPathStyle)
		assert.Equal(t, "/{a}/b/{c}", newPath)
		assert.Equal(t, "", wildCardName)
	})

	t.Run("/{a}/b/{c}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}/b/{c}", SpringWeb.EchoPathStyle)
		assert.Equal(t, "/:a/b/:c", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}", SpringWeb.GinPathStyle)
		assert.Equal(t, "/:a/b/:c", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}", SpringWeb.JavaPathStyle)
		assert.Equal(t, "/{a}/b/{c}", newPath)
		assert.Equal(t, "", wildCardName)
	})

	t.Run("/:a/b/:c/*", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/:a/b/:c/*", SpringWeb.EchoPathStyle)
		assert.Equal(t, "/:a/b/:c/*", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c/*", SpringWeb.GinPathStyle)
		assert.Equal(t, "/:a/b/:c/*@_@", newPath)
		assert.Equal(t, "@_@", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c/*", SpringWeb.JavaPathStyle)
		assert.Equal(t, "/{a}/b/{c}/{*}", newPath)
		assert.Equal(t, "", wildCardName)
	})

	t.Run("/{a}/b/{c}/{*}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}/b/{c}/{*}", SpringWeb.EchoPathStyle)
		assert.Equal(t, "/:a/b/:c/*", newPath)
		assert.Equal(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{*}", SpringWeb.GinPathStyle)
		assert.Equal(t, "/:a/b/:c/*@_@", newPath)
		assert.Equal(t, "@_@", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{*}", SpringWeb.JavaPathStyle)
		assert.Equal(t, "/{a}/b/{c}/{*}", newPath)
		assert.Equal(t, "", wildCardName)
	})

	t.Run("/:a/b/:c/*e", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/:a/b/:c/*e", SpringWeb.EchoPathStyle)
		assert.Equal(t, "/:a/b/:c/*", newPath)
		assert.Equal(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c/*e", SpringWeb.GinPathStyle)
		assert.Equal(t, "/:a/b/:c/*e", newPath)
		assert.Equal(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c/*e", SpringWeb.JavaPathStyle)
		assert.Equal(t, "/{a}/b/{c}/{*:e}", newPath)
		assert.Equal(t, "e", wildCardName)
	})

	t.Run("/{a}/b/{c}/{*:e}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}/b/{c}/{*:e}", SpringWeb.EchoPathStyle)
		assert.Equal(t, "/:a/b/:c/*", newPath)
		assert.Equal(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{*:e}", SpringWeb.GinPathStyle)
		assert.Equal(t, "/:a/b/:c/*e", newPath)
		assert.Equal(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{*:e}", SpringWeb.JavaPathStyle)
		assert.Equal(t, "/{a}/b/{c}/{*:e}", newPath)
		assert.Equal(t, "e", wildCardName)
	})

	t.Run("/{a}/b/{c}/{e:*}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}/b/{c}/{e:*}", SpringWeb.EchoPathStyle)
		assert.Equal(t, "/:a/b/:c/*", newPath)
		assert.Equal(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{e:*}", SpringWeb.GinPathStyle)
		assert.Equal(t, "/:a/b/:c/*e", newPath)
		assert.Equal(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{e:*}", SpringWeb.JavaPathStyle)
		assert.Equal(t, "/{a}/b/{c}/{*:e}", newPath)
		assert.Equal(t, "e", wildCardName)
	})
}
