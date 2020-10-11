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

package abort

import (
	"strconv"

	"github.com/go-spring/spring-web"
)

type StringArray struct {
	Data []string
}

func (s *StringArray) Push(str string) {
	s.Data = append(s.Data, str)
}

type PushFilter struct {
	num   int
	abort bool
	array *StringArray
}

func NewPushFilter(num int, abort bool, array *StringArray) *PushFilter {
	return &PushFilter{num: num, abort: abort, array: array}
}

func (f *PushFilter) Invoke(ctx SpringWeb.WebContext, chain SpringWeb.FilterChain) {

	defer func() { // 返回时没有中断则添加一个数
		if !ctx.IsAborted() {
			f.array.Push(strconv.Itoa(f.num))
		}
	}()

	if f.abort { // 中断处理过程
		ctx.Abort()
		return
	}

	f.array.Push(strconv.Itoa(f.num))
	chain.Next(ctx)
}
