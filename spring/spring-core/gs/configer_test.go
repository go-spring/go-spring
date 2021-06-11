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

package gs

import (
	"container/list"
	"fmt"
	"testing"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/sort"
)

func TestSortConfigers(t *testing.T) {

	t.Run("found cycle", func(t *testing.T) {
		assert.Panic(t, func() {

			f2 := new(Configer).After("f5").WithName("f2")
			f5 := new(Configer).After("f2").WithName("f5")
			f7 := new(Configer).Before("f2")

			configers := list.New()
			configers.PushBack(f5)
			configers.PushBack(f2)
			configers.PushBack(f7)

			sorted := sort.Triple(configers, getBeforeList)

			for e := sorted.Front(); e != nil; e = e.Next() {
				fmt.Println(e.Value.(*Configer).name)
			}

		}, "found sorting cycle")
	})

	t.Run("sorted", func(t *testing.T) {

		f2 := new(Configer).WithName("f2")
		f5 := new(Configer).WithName("f5").After("f2")
		f7 := new(Configer).Before("f2")

		configers := list.New()
		configers.PushBack(f5)
		configers.PushBack(f2)
		configers.PushBack(f7)

		sorted := sort.Triple(configers, getBeforeList)

		expect := list.New()
		expect.PushBack(f7)
		expect.PushBack(f2)
		expect.PushBack(f5)

		assert.Equal(t, sorted, expect)
	})
}
