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
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
)

func init() {
	SpringBoot.RegisterNameBean("server", NewStringFilter("server"))
	SpringBoot.RegisterNameBean("container", NewStringFilter("container"))
	SpringBoot.RegisterNameBean("router", NewStringFilter("router"))
	SpringBoot.RegisterNameBean("router//ok", NewStringFilter("router//ok"))
	SpringBoot.RegisterNameBean("router//echo", NewStringFilter("router//echo"))
}

type StringFilter struct {
	_ SpringWeb.Filter `export:""`

	s string
}

func NewStringFilter(s string) *StringFilter {
	return &StringFilter{s: s}
}

func (f *StringFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {

	defer ctx.LogInfo("after ", f.s)
	ctx.LogInfo("before ", f.s)

	chain.Next(ctx)
}
