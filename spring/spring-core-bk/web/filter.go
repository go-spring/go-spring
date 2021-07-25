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

package web

import (
	"regexp"
)

// Filter 过滤器接口
type Filter interface {
	// Invoke 通过 chain.Next() 驱动链条向后执行
	Invoke(ctx Context, chain FilterChain)
}

// FuncFilter 函数实现的过滤器
type FuncFilter func(ctx Context, chain FilterChain)

func (f FuncFilter) Invoke(ctx Context, chain FilterChain) {
	f(ctx, chain)
}

// handlerFilter 包装 Web 处理接口的过滤器
type handlerFilter struct {
	fn Handler
}

// HandlerFilter 把 Web 处理接口转换成过滤器
func HandlerFilter(fn Handler) Filter {
	return &handlerFilter{fn: fn}
}

func (h *handlerFilter) Invoke(ctx Context, _ FilterChain) {
	h.fn.Invoke(ctx)
}

// FilterChain 过滤器链条接口
type FilterChain interface {
	Next(ctx Context)
}

// DefaultFilterChain 默认的过滤器链条
type DefaultFilterChain struct {
	filters []Filter // 过滤器列表
	next    int      // 下一个等待执行的过滤器的序号
}

// NewDefaultFilterChain DefaultFilterChain 的构造函数
func NewDefaultFilterChain(filters []Filter) *DefaultFilterChain {
	return &DefaultFilterChain{filters: filters}
}

func (chain *DefaultFilterChain) Next(ctx Context) {

	// 链条执行到此结束
	if chain.next >= len(chain.filters) {
		return
	}

	// 执行下一个过滤器
	f := chain.filters[chain.next]
	chain.next++
	f.Invoke(ctx, chain)
}

type urlPatterns struct {
	m map[*regexp.Regexp][]Filter
}

func (p *urlPatterns) Get(path string) []Filter {
	for pattern, filters := range p.m {
		if pattern.MatchString(path) {
			return filters
		}
	}
	return nil
}

// URLPatterns 根据 Filter 的 URL 匹配表达式进行分组。
func URLPatterns(filters []Filter) (*urlPatterns, error) {

	filterMap := make(map[string][]Filter)
	for _, filter := range filters {
		var patterns []string
		if p, ok := filter.(interface{ URLPatterns() []string }); ok {
			patterns = p.URLPatterns()
		} else {
			patterns = []string{"/*"}
		}
		for _, pattern := range patterns {
			filterMap[pattern] = append(filterMap[pattern], filter)
		}
	}

	filterPatterns := make(map[*regexp.Regexp][]Filter)
	for pattern, filter := range filterMap {
		exp, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		filterPatterns[exp] = filter
	}
	return &urlPatterns{m: filterPatterns}, nil
}
