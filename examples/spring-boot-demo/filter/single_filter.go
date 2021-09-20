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

	"github.com/go-spring/spring-boost/log"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
)

func init() {
	gs.Object(new(SingleBeanFilter)).Export((*web.Filter)(nil))
}

type SingleBeanFilter struct {
	DefaultValue string `value:"${default-value:=default}"`
}

func (f *SingleBeanFilter) URLPatterns() []string {
	return []string{"/api/*"}
}

func (f *SingleBeanFilter) Invoke(ctx web.Context, chain web.FilterChain) {
	if f.DefaultValue != "app-test" {
		panic(fmt.Errorf("${default-value} expect 'app-test' but '%s'", f.DefaultValue))
	}
	log.Ctx(ctx.Context()).Info("::SingleBeanFilter")
	chain.Next(ctx)
}
