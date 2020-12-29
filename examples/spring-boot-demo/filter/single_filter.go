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
	"fmt"

	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-web"
)

func init() {
	// 这种方式可以避免使用 export 语法，就像 StringFilter 和 NumberFilter 那样。
	SpringBoot.RegisterFilter(SpringCore.ObjectBean(new(SingleBeanFilter)))
}

type SingleBeanFilter struct {
	DefaultValue string `value:"${default-value:=default}"`
}

func (f *SingleBeanFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {
	if f.DefaultValue != "app-test" {
		panic(fmt.Errorf("${default-value} expect 'app-test' but '%s'", f.DefaultValue))
	}
	SpringLogger.WithContext(ctx.Context()).Info("::SingleBeanFilter")
	chain.Next(ctx)
}
