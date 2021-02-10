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
	"fmt"
	"net/http"
	"testing"

	"github.com/go-spring/spring-core/util"
	"github.com/go-spring/spring-core/web"
)

// cacheMethods
var cacheMethods = map[uint32][]string{
	web.MethodGet:     {http.MethodGet},
	web.MethodHead:    {http.MethodHead},
	web.MethodPost:    {http.MethodPost},
	web.MethodPut:     {http.MethodPut},
	web.MethodPatch:   {http.MethodPatch},
	web.MethodDelete:  {http.MethodDelete},
	web.MethodConnect: {http.MethodConnect},
	web.MethodOptions: {http.MethodOptions},
	web.MethodTrace:   {http.MethodTrace},
	web.MethodGetPost: {http.MethodGet, http.MethodPost},
	web.MethodAny:     {http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace},
}

func GetMethodViaCache(method uint32) []string {
	if r, ok := cacheMethods[method]; ok {
		return r
	}
	return web.GetMethod(method)
}

func BenchmarkGetMethod(b *testing.B) {
	// 测试结论：使用缓存不一定能提高效率

	b.Run("1", func(b *testing.B) {
		web.GetMethod(web.MethodGet)
	})

	b.Run("cache-1", func(b *testing.B) {
		GetMethodViaCache(web.MethodGet)
	})

	b.Run("2", func(b *testing.B) {
		web.GetMethod(web.MethodGet | web.MethodHead)
	})

	b.Run("cache-2", func(b *testing.B) {
		GetMethodViaCache(web.MethodGet | web.MethodHead)
	})

	b.Run("3", func(b *testing.B) {
		web.GetMethod(web.MethodGet | web.MethodHead | web.MethodPost)
	})

	b.Run("cache-3", func(b *testing.B) {
		GetMethodViaCache(web.MethodGet | web.MethodHead | web.MethodPost)
	})

	b.Run("4", func(b *testing.B) {
		web.GetMethod(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut)
	})

	b.Run("cache-4", func(b *testing.B) {
		GetMethodViaCache(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut)
	})

	b.Run("5", func(b *testing.B) {
		web.GetMethod(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch)
	})

	b.Run("cache-5", func(b *testing.B) {
		GetMethodViaCache(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch)
	})

	b.Run("6", func(b *testing.B) {
		web.GetMethod(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch | web.MethodDelete)
	})

	b.Run("cache-6", func(b *testing.B) {
		GetMethodViaCache(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch | web.MethodDelete)
	})

	b.Run("7", func(b *testing.B) {
		web.GetMethod(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch | web.MethodDelete | web.MethodConnect)
	})

	b.Run("cache-7", func(b *testing.B) {
		GetMethodViaCache(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch | web.MethodDelete | web.MethodConnect)
	})

	b.Run("8", func(b *testing.B) {
		web.GetMethod(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch | web.MethodDelete | web.MethodConnect | web.MethodOptions)
	})

	b.Run("cache-8", func(b *testing.B) {
		GetMethodViaCache(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch | web.MethodDelete | web.MethodConnect | web.MethodOptions)
	})

	b.Run("9", func(b *testing.B) {
		web.GetMethod(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch | web.MethodDelete | web.MethodConnect | web.MethodOptions | web.MethodTrace)
	})

	b.Run("cache-9", func(b *testing.B) {
		GetMethodViaCache(web.MethodGet | web.MethodHead | web.MethodPost | web.MethodPut | web.MethodPatch | web.MethodDelete | web.MethodConnect | web.MethodOptions | web.MethodTrace)
	})
}

func TestMapper_Key(t *testing.T) {
	fmt.Println(web.NewMapper(web.MethodAny, "/", nil, nil).Key())
	fmt.Println(web.NewMapper(web.MethodGet, "/", nil, nil).Key())
	fmt.Println(web.NewMapper(web.MethodGetPost, "/", nil, nil).Key())
}

func TestRouter_Route(t *testing.T) {
	root := web.NewRootRouter().Route("/root", web.LoggerFilter, web.LoggerFilter)

	get := root.GetMapping("/get", nil, web.LoggerFilter, web.LoggerFilter)
	util.AssertEqual(t, get.Path(), "/root/get")
	util.AssertEqual(t, len(get.Filters()), 4)

	sub := root.Route("/sub", web.LoggerFilter, web.LoggerFilter)
	subGet := sub.GetMapping("/get", nil, web.LoggerFilter, web.LoggerFilter)
	util.AssertEqual(t, subGet.Path(), "/root/sub/get")
	util.AssertEqual(t, len(subGet.Filters()), 6)

	subSub := sub.Route("/sub", web.LoggerFilter, web.LoggerFilter)
	subSubGet := subSub.GetMapping("/get", nil, web.LoggerFilter, web.LoggerFilter)
	util.AssertEqual(t, subSubGet.Path(), "/root/sub/sub/get")
	util.AssertEqual(t, len(subSubGet.Filters()), 8)
}
