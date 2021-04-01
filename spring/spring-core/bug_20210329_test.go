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

package SpringCore_test

import (
	"testing"

	"github.com/go-spring/spring-core"
)

type circularA struct {
	b *circularB
}

func newCircularA(b *circularB) *circularA {
	return &circularA{b: b}
}

type circularB struct {
	A *circularA `autowire:",lazy"`
}

func newCircularB() *circularB {
	return new(circularB)
}

func Test20210329(t *testing.T) {
	for i := 0; i < 20; i++ {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterNameBeanFn("1", newCircularA)
		ctx.RegisterNameBeanFn("2", newCircularB)
		ctx.AutoWireBeans()
	}
}
