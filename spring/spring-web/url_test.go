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

	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
)

func TestToPathStyle(t *testing.T) {

	t.Run("/:a", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/:a", SpringWeb.EchoPathStyle)
		SpringUtils.AssertEqual(t, "/:a", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a", SpringWeb.GinPathStyle)
		SpringUtils.AssertEqual(t, "/:a", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a", SpringWeb.JavaPathStyle)
		SpringUtils.AssertEqual(t, "/{a}", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
	})

	t.Run("/{a}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}", SpringWeb.EchoPathStyle)
		SpringUtils.AssertEqual(t, "/:a", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}", SpringWeb.GinPathStyle)
		SpringUtils.AssertEqual(t, "/:a", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}", SpringWeb.JavaPathStyle)
		SpringUtils.AssertEqual(t, "/{a}", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
	})

	t.Run("/:a/b/:c", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/:a/b/:c", SpringWeb.EchoPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c", SpringWeb.GinPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c", SpringWeb.JavaPathStyle)
		SpringUtils.AssertEqual(t, "/{a}/b/{c}", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
	})

	t.Run("/{a}/b/{c}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}/b/{c}", SpringWeb.EchoPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}", SpringWeb.GinPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}", SpringWeb.JavaPathStyle)
		SpringUtils.AssertEqual(t, "/{a}/b/{c}", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
	})

	t.Run("/:a/b/:c/*", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/:a/b/:c/*", SpringWeb.EchoPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c/*", SpringWeb.GinPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*@_@", newPath)
		SpringUtils.AssertEqual(t, "@_@", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c/*", SpringWeb.JavaPathStyle)
		SpringUtils.AssertEqual(t, "/{a}/b/{c}/{*}", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
	})

	t.Run("/{a}/b/{c}/{*}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}/b/{c}/{*}", SpringWeb.EchoPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{*}", SpringWeb.GinPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*@_@", newPath)
		SpringUtils.AssertEqual(t, "@_@", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{*}", SpringWeb.JavaPathStyle)
		SpringUtils.AssertEqual(t, "/{a}/b/{c}/{*}", newPath)
		SpringUtils.AssertEqual(t, "", wildCardName)
	})

	t.Run("/:a/b/:c/*e", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/:a/b/:c/*e", SpringWeb.EchoPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*", newPath)
		SpringUtils.AssertEqual(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c/*e", SpringWeb.GinPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*e", newPath)
		SpringUtils.AssertEqual(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/:a/b/:c/*e", SpringWeb.JavaPathStyle)
		SpringUtils.AssertEqual(t, "/{a}/b/{c}/{*:e}", newPath)
		SpringUtils.AssertEqual(t, "e", wildCardName)
	})

	t.Run("/{a}/b/{c}/{*:e}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}/b/{c}/{*:e}", SpringWeb.EchoPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*", newPath)
		SpringUtils.AssertEqual(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{*:e}", SpringWeb.GinPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*e", newPath)
		SpringUtils.AssertEqual(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{*:e}", SpringWeb.JavaPathStyle)
		SpringUtils.AssertEqual(t, "/{a}/b/{c}/{*:e}", newPath)
		SpringUtils.AssertEqual(t, "e", wildCardName)
	})

	t.Run("/{a}/b/{c}/{e:*}", func(t *testing.T) {
		newPath, wildCardName := SpringWeb.ToPathStyle("/{a}/b/{c}/{e:*}", SpringWeb.EchoPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*", newPath)
		SpringUtils.AssertEqual(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{e:*}", SpringWeb.GinPathStyle)
		SpringUtils.AssertEqual(t, "/:a/b/:c/*e", newPath)
		SpringUtils.AssertEqual(t, "e", wildCardName)
		newPath, wildCardName = SpringWeb.ToPathStyle("/{a}/b/{c}/{e:*}", SpringWeb.JavaPathStyle)
		SpringUtils.AssertEqual(t, "/{a}/b/{c}/{*:e}", newPath)
		SpringUtils.AssertEqual(t, "e", wildCardName)
	})
}
