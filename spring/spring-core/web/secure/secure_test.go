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

package secure

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecure(t *testing.T) {
	options := Options{
		//  AllowedHosts: []string{"www.baidu.com"},
		//  SSLRedirect: true,
		BrowserXssFilter: true,
		FrameDeny:        true,
		IsDevelopment:    true,
	}

	s := newSecure(options)

	w, r := getReq("get")

	err := s.process(w, r)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Printf("httpcode: %d", w.Code)
	}

}

func getReq(method string) (*httptest.ResponseRecorder, *http.Request) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest(method, "http://t.dev:8080", nil)
	return w, r
}
