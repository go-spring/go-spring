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

package SpringWeb

import (
	"net/http"
)

const (
	MethodGet     = 0x0001 // "GET"
	MethodHead    = 0x0002 // "HEAD"
	MethodPost    = 0x0004 // "POST"
	MethodPut     = 0x0008 // "PUT"
	MethodPatch   = 0x0010 // "PATCH"
	MethodDelete  = 0x0020 // "DELETE"
	MethodConnect = 0x0040 // "CONNECT"
	MethodOptions = 0x0080 // "OPTIONS"
	MethodTrace   = 0x0100 // "TRACE"
	MethodAny     = 0xffff
	MethodGetPost = MethodGet | MethodPost
)

var methods = map[uint32]string{
	MethodGet:     http.MethodGet,
	MethodHead:    http.MethodHead,
	MethodPost:    http.MethodPost,
	MethodPut:     http.MethodPut,
	MethodPatch:   http.MethodPatch,
	MethodDelete:  http.MethodDelete,
	MethodConnect: http.MethodConnect,
	MethodOptions: http.MethodOptions,
	MethodTrace:   http.MethodTrace,
}

// GetMethod 返回 method 对应的 HTTP 方法
func GetMethod(method uint32) []string {
	var r []string
	for k, v := range methods {
		if method&k == k {
			r = append(r, v)
		}
	}
	return r
}
