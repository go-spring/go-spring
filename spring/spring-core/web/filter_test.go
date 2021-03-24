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
	"testing"

	"github.com/go-spring/spring-core/web"
)

func TestFuncFilter(t *testing.T) {

	funcFilter := web.FuncFilter(func(ctx web.Context, chain web.FilterChain) {
		fmt.Println("@FuncFilter")
		chain.Next(ctx)
	})

	handlerFilter := web.HandlerFilter(web.FUNC(func(ctx web.Context) {
		fmt.Println("@HandlerFilter")
	}))

	web.NewDefaultFilterChain([]web.Filter{funcFilter, handlerFilter}).Next(nil)
}
