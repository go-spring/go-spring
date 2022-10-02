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

package assert_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
)

func TestString_EqualFold(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.String(g, "hello, world!").EqualFold("Hello, World!")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"'hello, world!' doesn't equal fold to 'xxx'"})
		assert.String(g, "hello, world!").EqualFold("xxx")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"'hello, world!' doesn't equal fold to 'xxx'; param (index=0)"})
		assert.String(g, "hello, world!").EqualFold("xxx", "param (index=0)")
	})
}

func TestString_HasPrefix(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.String(g, "hello, world!").HasPrefix("hello")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"'hello, world!' doesn't have prefix 'xxx'"})
		assert.String(g, "hello, world!").HasPrefix("xxx")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"'hello, world!' doesn't have prefix 'xxx'; param (index=0)"})
		assert.String(g, "hello, world!").HasPrefix("xxx", "param (index=0)")
	})
}

func TestString_HasSuffix(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.String(g, "hello, world!").HasSuffix("world!")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"'hello, world!' doesn't have suffix 'xxx'"})
		assert.String(g, "hello, world!").HasSuffix("xxx")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"'hello, world!' doesn't have suffix 'xxx'; param (index=0)"})
		assert.String(g, "hello, world!").HasSuffix("xxx", "param (index=0)")
	})
}

func TestString_HasSubString(t *testing.T) {

	Case(t, func(g *assert.MockT) {
		assert.String(g, "hello, world!").HasSubStr("hello")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"'hello, world!' doesn't contain substr 'xxx'"})
		assert.String(g, "hello, world!").HasSubStr("xxx")
	})

	Case(t, func(g *assert.MockT) {
		g.EXPECT().Error([]interface{}{"'hello, world!' doesn't contain substr 'xxx'; param (index=0)"})
		assert.String(g, "hello, world!").HasSubStr("xxx", "param (index=0)")
	})
}
