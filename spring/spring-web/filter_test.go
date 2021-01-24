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

	"github.com/go-spring/spring-web"
)

func TestFuncFilter(t *testing.T) {

	funcFilter := SpringWeb.FuncFilter(func(ctx SpringWeb.Context, chain SpringWeb.FilterChain) {
		fmt.Println("@FuncFilter")
		chain.Next(ctx)
	})

	handlerFilter := SpringWeb.HandlerFilter(SpringWeb.FUNC(func(ctx SpringWeb.Context) {
		fmt.Println("@HandlerFilter")
	}))

	SpringWeb.NewDefaultFilterChain([]SpringWeb.Filter{funcFilter, handlerFilter}).Next(nil)
}
