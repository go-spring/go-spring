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

package binding_test

import (
	"net/url"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/web/binding"
)

type MockRequest struct {
	contentType string
	headers     map[string]string
	queryParams map[string]string
	pathParams  map[string]string
	formParams  url.Values
	requestBody string
}

var _ binding.Request = &MockRequest{}

func (r *MockRequest) ContentType() string {
	return r.contentType
}

func (r *MockRequest) Header(key string) string {
	return r.headers[key]
}

func (r *MockRequest) QueryParam(name string) string {
	return r.queryParams[name]
}

func (r *MockRequest) PathParam(name string) string {
	return r.pathParams[name]
}

func (r *MockRequest) FormParams() (url.Values, error) {
	return r.formParams, nil
}

func (r *MockRequest) RequestBody() ([]byte, error) {
	return []byte(r.requestBody), nil
}

type ScopeBindParam struct {
	A string `uri:"a"`
	B string `path:"b"`
	C string `uri:"c" query:"c"`
	D string `param:"d"`
	E string `query:"e" header:"e"`
}

func TestScopeBind(t *testing.T) {

	ctx := &MockRequest{
		headers: map[string]string{
			"e": "6",
		},
		queryParams: map[string]string{
			"c": "3",
			"d": "4",
			"e": "5",
		},
		pathParams: map[string]string{
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
	err := binding.Bind(&p, ctx)
	assert.Nil(t, err)
	assert.Equal(t, p, expect)
}
