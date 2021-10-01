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

package atomic_test

import (
	"reflect"
	"sync"
	"testing"
	"unsafe"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/atomic"
)

func TestSize(t *testing.T) {
	assert.Equal(t, unsafe.Sizeof(int64(0)), uintptr(8))
	assert.Equal(t, unsafe.Sizeof(atomic.Int64{}), uintptr(8))
}

func TestInt64(t *testing.T) {

	var i64 atomic.Int64
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			i64.Add(1)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			i64.Add(2)
		}
	}()
	wg.Wait()
	assert.Equal(t, int64(30), i64.Load())

	wg.Add(2)
	go func() {
		defer wg.Done()
		i64.Store(3)
	}()
	go func() {
		defer wg.Done()
		i64.Store(4)
	}()
	wg.Wait()
	assert.True(t, i64.Load() == 3 || i64.Load() == 4)
}

func TestReflectInt64(t *testing.T) {

	// s 必须分配在堆上
	s := new(struct {
		I atomic.Int64
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		addr := reflect.ValueOf(s).Elem().Field(0).Addr()
		v, ok := addr.Interface().(*atomic.Int64)
		assert.True(t, ok)
		for i := 0; i < 10; i++ {
			v.Add(1)
		}
	}()
	go func() {
		defer wg.Done()
		addr := reflect.ValueOf(s).Elem().Field(0).Addr()
		v, ok := addr.Interface().(*atomic.Int64)
		assert.True(t, ok)
		for i := 0; i < 10; i++ {
			v.Add(2)
		}
	}()
	wg.Wait()
	assert.Equal(t, int64(30), s.I.Load())
}
