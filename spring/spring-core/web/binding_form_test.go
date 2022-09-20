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
	"net/url"
	"strings"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/web"
)

type FormBindParamCommon struct {
	A string   `form:"a"`
	B []string `form:"b"`
}

type FormBindParam struct {
	FormBindParamCommon
	C int   `form:"c"`
	D []int `form:"d"`
}

func TestBindForm(t *testing.T) {

	data := url.Values{
		"a": {"1"},
		"b": {"2", "3"},
		"c": {"4"},
		"d": {"5", "6"},
	}
	target := "http://localhost:8080/1/2"
	body := strings.NewReader(data.Encode())
	req := httptest.NewRequest("POST", target, body)
	req.Header.Set(web.HeaderContentType, web.MIMEApplicationForm)
	ctx := &MockContext{
		BaseContext: web.NewBaseContext("/:a/:b", nil, req, nil),
	}

	expect := FormBindParam{
		FormBindParamCommon: FormBindParamCommon{
			A: "1",
			B: []string{"2", "3"},
		},
		C: 4,
		D: []int{5, 6},
	}

	var p FormBindParam
	err := web.Bind(&p, ctx)
	assert.Nil(t, err)
	assert.Equal(t, p, expect)
}
