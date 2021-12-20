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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/web"
)

func TestRequestIDFilter(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:8080/", nil)
	w := httptest.NewRecorder()
	ctx := web.NewHttpContext(nil, w, r)
	f := web.NewRequestIDFilter(web.RequestIDConfig{
		Generator: func() string { return "0d9ad123-327f-bde5-14b4-8f93c36c3546" },
	})
	web.NewDefaultFilterChain([]web.Filter{f}).Next(ctx)
	assert.Equal(t, ctx.GetHeader(web.HeaderXRequestID), "0d9ad123-327f-bde5-14b4-8f93c36c3546")
}
