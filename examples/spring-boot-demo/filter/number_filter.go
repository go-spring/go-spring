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

package filter

import (
	"container/list"

	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
)

func init() {
	l := list.New()
	SpringBoot.RegisterNameBean("f2", NewNumberFilter(2, l))
	SpringBoot.RegisterNameBean("f5", NewNumberFilter(5, l))
	SpringBoot.RegisterNameBean("f7", NewNumberFilter(7, l))
}

type NumberFilter struct {
	_ SpringWeb.Filter `export:""`

	l *list.List
	N int
}

func NewNumberFilter(n int, l *list.List) *NumberFilter {
	return &NumberFilter{
		l: l,
		N: n,
	}
}

func (f *NumberFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {

	defer func() {
		ctx.LogInfo("::after", f.N)
		f.l.PushBack(f.N)
	}()

	ctx.LogInfo("::before", f.N)
	f.l.PushBack(f.N)

	chain.Next(ctx)
}
