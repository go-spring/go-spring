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
	"fmt"
	"net/http"
	"testing"

	"github.com/go-spring/spring-utils"
)

// cacheMethods
var cacheMethods = map[uint32][]string{
	MethodGet:     {http.MethodGet},
	MethodHead:    {http.MethodHead},
	MethodPost:    {http.MethodPost},
	MethodPut:     {http.MethodPut},
	MethodPatch:   {http.MethodPatch},
	MethodDelete:  {http.MethodDelete},
	MethodConnect: {http.MethodConnect},
	MethodOptions: {http.MethodOptions},
	MethodTrace:   {http.MethodTrace},
	MethodGetPost: {http.MethodGet, http.MethodPost},
	MethodAny:     {http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace},
}

func GetMethodViaCache(method uint32) []string {
	if r, ok := cacheMethods[method]; ok {
		return r
	}
	return GetMethod(method)
}

func BenchmarkGetMethod(b *testing.B) {
	// 测试结论：使用缓存不一定能提高效率

	b.Run("1", func(b *testing.B) {
		GetMethod(MethodGet)
	})

	b.Run("cache-1", func(b *testing.B) {
		GetMethodViaCache(MethodGet)
	})

	b.Run("2", func(b *testing.B) {
		GetMethod(MethodGet | MethodHead)
	})

	b.Run("cache-2", func(b *testing.B) {
		GetMethodViaCache(MethodGet | MethodHead)
	})

	b.Run("3", func(b *testing.B) {
		GetMethod(MethodGet | MethodHead | MethodPost)
	})

	b.Run("cache-3", func(b *testing.B) {
		GetMethodViaCache(MethodGet | MethodHead | MethodPost)
	})

	b.Run("4", func(b *testing.B) {
		GetMethod(MethodGet | MethodHead | MethodPost | MethodPut)
	})

	b.Run("cache-4", func(b *testing.B) {
		GetMethodViaCache(MethodGet | MethodHead | MethodPost | MethodPut)
	})

	b.Run("5", func(b *testing.B) {
		GetMethod(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch)
	})

	b.Run("cache-5", func(b *testing.B) {
		GetMethodViaCache(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch)
	})

	b.Run("6", func(b *testing.B) {
		GetMethod(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch | MethodDelete)
	})

	b.Run("cache-6", func(b *testing.B) {
		GetMethodViaCache(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch | MethodDelete)
	})

	b.Run("7", func(b *testing.B) {
		GetMethod(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch | MethodDelete | MethodConnect)
	})

	b.Run("cache-7", func(b *testing.B) {
		GetMethodViaCache(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch | MethodDelete | MethodConnect)
	})

	b.Run("8", func(b *testing.B) {
		GetMethod(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch | MethodDelete | MethodConnect | MethodOptions)
	})

	b.Run("cache-8", func(b *testing.B) {
		GetMethodViaCache(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch | MethodDelete | MethodConnect | MethodOptions)
	})

	b.Run("9", func(b *testing.B) {
		GetMethod(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch | MethodDelete | MethodConnect | MethodOptions | MethodTrace)
	})

	b.Run("cache-9", func(b *testing.B) {
		GetMethodViaCache(MethodGet | MethodHead | MethodPost | MethodPut | MethodPatch | MethodDelete | MethodConnect | MethodOptions | MethodTrace)
	})
}

func TestMapper_Key(t *testing.T) {
	fmt.Println(NewMapper(MethodAny, "/", nil, nil).Key())
	fmt.Println(NewMapper(MethodGet, "/", nil, nil).Key())
	fmt.Println(NewMapper(MethodGetPost, "/", nil, nil).Key())
}

func TestRouter_Route(t *testing.T) {
	root := NewRootRouter().Route("/root", loggerFilter, loggerFilter)

	get := root.GetMapping("/get", nil, loggerFilter, loggerFilter)
	SpringUtils.AssertEqual(t, get.path, "/root/get")
	SpringUtils.AssertEqual(t, len(get.filters), 4)

	sub := root.Route("/sub", loggerFilter, loggerFilter)
	subGet := sub.GetMapping("/get", nil, loggerFilter, loggerFilter)
	SpringUtils.AssertEqual(t, subGet.path, "/root/sub/get")
	SpringUtils.AssertEqual(t, len(subGet.filters), 6)

	subSub := sub.Route("/sub", loggerFilter, loggerFilter)
	subSubGet := subSub.GetMapping("/get", nil, loggerFilter, loggerFilter)
	SpringUtils.AssertEqual(t, subSubGet.path, "/root/sub/sub/get")
	SpringUtils.AssertEqual(t, len(subSubGet.filters), 8)
}
