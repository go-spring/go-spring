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

package cors

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCors(t *testing.T) {
	config := AllowAllConfig()
	c := newCors(config)
	fmt.Printf("isMethodAllowed: %v \n", c.isMethodAllowed("GET"))

	_, r := getReq("GET")
	fmt.Printf("isOriginAllowed: %v \n", c.isOriginAllowed(r, "http//127.0.0.1:9090"))

	options := Options{
		AllowedOrigins: []string{"http://127.0.0.1:8081"},
		AllowOriginRequestFunc: func(r *http.Request, origin string) bool {
			return false
		},
		AllowedHeaders: []string{"*"},
		AllowedMethods: []string{
			http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions,
		},
		AllowCredentials: false,
	}
	c2 := newCors(options)
	fmt.Printf("c2 isMethodAllowed: %v \n", c2.isMethodAllowed("GET"))
	fmt.Printf("c2 isMethodAllowed: %v \n", c2.isMethodAllowed("POST"))
	_, r = getReq("POST")
	fmt.Printf("c2 isOriginAllowed: %v \n", c2.isOriginAllowed(r, "http//127.0.0.1:9091"))
}

func TestParseHeaderList(t *testing.T) {
	list := parseHeaderList("token,Aut")
	fmt.Println(list)
	list = parseHeaderList("token, Aut")
	fmt.Println(list)
	list = parseHeaderList("token,aut")
	fmt.Println(list)
}

func getReq(method string) (*httptest.ResponseRecorder, *http.Request) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest(method, "http://t.dev:8080", nil)
	return w, r
}
