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

package SpringCore

import (
	"container/list"
	"fmt"
	"testing"

	"github.com/go-spring/spring-core/internal/sort"
	"github.com/go-spring/spring-utils"
)

func TestSortConfigers(t *testing.T) {

	t.Run("found cycle", func(t *testing.T) {
		SpringUtils.AssertPanic(t, func() {

			f2 := newConfiger("f2", func() {}, []string{}).After("f5")
			f5 := newConfiger("f5", func() {}, []string{}).After("f2")
			f7 := newConfiger("", func() {}, []string{}).Before("f2")

			configers := list.New()
			configers.PushBack(f5)
			configers.PushBack(f2)
			configers.PushBack(f7)

			sorted := sort.TripleSorting(configers, getBeforeConfigers)

			for e := sorted.Front(); e != nil; e = e.Next() {
				fmt.Println(e.Value.(*Configer).name)
			}

		}, "found sorting cycle")
	})

	t.Run("sorted", func(t *testing.T) {

		f2 := newConfiger("f2", func() {}, []string{})
		f5 := newConfiger("f5", func() {}, []string{}).After("f2")
		f7 := newConfiger("", func() {}, []string{}).Before("f2")

		configers := list.New()
		configers.PushBack(f5)
		configers.PushBack(f2)
		configers.PushBack(f7)

		sorted := sort.TripleSorting(configers, getBeforeConfigers)

		expect := list.New()
		expect.PushBack(f7)
		expect.PushBack(f2)
		expect.PushBack(f5)

		SpringUtils.AssertEqual(t, sorted, expect)
	})
}
