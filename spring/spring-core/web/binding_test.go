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
	"net/http/httptest"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/web"
)

type ScopeBindParam struct {
	A string `uri:"a"`
	B string `path:"b"`
	C string `uri:"c" query:"c"`
	D string `param:"d"`
	E string `query:"e" header:"e"`
}

type MockContext struct {
	*web.BaseContext
	uriParam map[string]string
}

func (ctx *MockContext) PathParam(name string) string {
	return ctx.uriParam[name]
}

func TestScopeBind(t *testing.T) {

	target := "http://localhost:8080/1/2?c=3&d=4&e=5"
	req := httptest.NewRequest("GET", target, nil)
	req.Header.Set("e", "6")

	ctx := &MockContext{
		BaseContext: web.NewBaseContext("/:a/:b", nil, req, nil),
		uriParam: map[string]string{
			"a": "1",
			"b": "2",
		},
	}

	expect := ScopeBindParam{
		A: "1",
		B: "2",
		C: "3",
		D: "4",
		E: "6",
	}

	var p ScopeBindParam
	err := web.Bind(&p, ctx)
	assert.Nil(t, err)
	assert.Equal(t, p, expect)
}
