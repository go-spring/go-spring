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

package SpringWeb_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/go-spring/spring-web"
)

type NumberFilter struct {
	n int
}

func NewNumberFilter(n int) *NumberFilter {
	return &NumberFilter{
		n: n,
	}
}

func (f *NumberFilter) Invoke(ctx SpringWeb.WebContext, chain *SpringWeb.FilterChain) {
	defer fmt.Println("::after", f.n)
	fmt.Println("::before", f.n)
	chain.Next(ctx)
}

func TestFilterChain(t *testing.T) {
	chain := SpringWeb.NewFilterChain(NewNumberFilter(2), NewNumberFilter(5))
	chain.Next(nil)
}
