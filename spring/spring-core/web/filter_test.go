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

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/web"
)

func TestFuncFilter(t *testing.T) {

	funcFilter := web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
		fmt.Println("@FuncFilter")
		chain.Next(ctx)
	}).URLPatterns("/func")

	handlerFilter := web.HandlerFilter(web.FUNC(func(ctx web.Context) {
		fmt.Println("@HandlerFilter")
	}))

	web.NewFilterChain([]web.Filter{funcFilter, handlerFilter}).Next(nil)
}

type Counter struct{}

func (ctr *Counter) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestWrapH(t *testing.T) {

	c := &Counter{}
	fmt.Println(util.FileLine(c.ServeHTTP))

	h := &Counter{}
	fmt.Println(util.FileLine(h.ServeHTTP))

	fmt.Println(web.WrapH(&Counter{}).FileLine())
}

func TestFilterChain_Continue(t *testing.T) {

	var stack []int
	var result [][]int

	filterImpl := func(i int) web.Filter {
		return web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
			stack = append(stack, i)
			result = append(result, stack)
			defer func() {
				stack = append([]int{}, stack[:len(stack)-1]...)
				result = append(result, stack)
			}()
			if i > 5 {
				return
			}
			if i%2 == 0 {
				chain.Next(ctx)
			} else {
				chain.Continue(ctx)
			}
		})
	}

	web.NewFilterChain([]web.Filter{
		filterImpl(1),
		filterImpl(2),
		filterImpl(3),
		filterImpl(4),
		filterImpl(5),
		filterImpl(6),
		filterImpl(7),
	}).Next(nil)

	assert.Equal(t, result, [][]int{
		{1},
		{},
		{2},
		{2, 3},
		{2},
		{2, 4},
		{2, 4, 5},
		{2, 4},
		{2, 4, 6},
		{2, 4},
		{2},
		{},
	})
}

func TestFilterChain_Next(t *testing.T) {

	var stack []int
	var result [][]int

	filterImpl := func(i int) web.Filter {
		return web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
			stack = append(stack, i)
			result = append(result, stack)
			defer func() {
				stack = append([]int{}, stack[:len(stack)-1]...)
				result = append(result, stack)
			}()
			if i > 5 {
				return
			}
			chain.Next(ctx)
		})
	}

	web.NewFilterChain([]web.Filter{
		filterImpl(1),
		filterImpl(2),
		filterImpl(3),
		filterImpl(4),
		filterImpl(5),
		filterImpl(6),
		filterImpl(7),
	}).Next(nil)

	assert.Equal(t, result, [][]int{
		{1},
		{1, 2},
		{1, 2, 3},
		{1, 2, 3, 4},
		{1, 2, 3, 4, 5},
		{1, 2, 3, 4, 5, 6},
		{1, 2, 3, 4, 5},
		{1, 2, 3, 4},
		{1, 2, 3},
		{1, 2},
		{1},
		{},
	})
}
