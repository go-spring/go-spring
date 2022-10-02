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

package knife_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/knife"
)

func TestKnife(t *testing.T) {
	ctx := context.Background()

	v, err := knife.Load(ctx, "a")
	assert.Error(t, err, "knife uninitialized")

	err = knife.Store(ctx, "a", "b")
	assert.Error(t, err, "knife uninitialized")

	_, _, err = knife.LoadOrStore(ctx, "a", 3)
	assert.Error(t, err, "knife uninitialized")

	ctx, cached := knife.New(ctx)
	assert.False(t, cached)

	v, err = knife.Load(ctx, "a")
	assert.Nil(t, err)
	assert.Nil(t, v)

	err = knife.Store(ctx, "a", "b")
	assert.Nil(t, err)

	err = knife.Store(ctx, "a", "c")
	assert.Error(t, err, "duplicate key \"a\"")

	v, err = knife.Load(ctx, "a")
	assert.Nil(t, err)
	assert.Equal(t, v, "b")

	{
		var keys []interface{}
		knife.Range(ctx, func(key, value interface{}) bool {
			keys = append(keys, key)
			return true
		})
		assert.Equal(t, keys, []interface{}{"a"})
	}

	ctx, cached = knife.New(ctx)
	assert.True(t, cached)

	v, err = knife.Load(ctx, "a")
	assert.Nil(t, err)
	assert.Equal(t, v, "b")

	knife.Delete(ctx, "a")
	{
		var keys []interface{}
		knife.Range(ctx, func(key, value interface{}) bool {
			keys = append(keys, key)
			return true
		})
		assert.Equal(t, keys, []interface{}(nil))
	}

	var (
		wg sync.WaitGroup
		m  sync.Mutex
		r  [][]interface{}
	)
	wg.Add(3)
	for i := 0; i < 3; i++ {
		val := strconv.Itoa(i)
		go func() {
			defer wg.Done()
			actual, loaded, _ := knife.LoadOrStore(ctx, "a", val)
			m.Lock()
			defer m.Unlock()
			r = append(r, []interface{}{actual, loaded})
		}()
	}
	wg.Wait()
	{
		count := 0
		var actual interface{}
		for i := 0; i < len(r); i++ {
			if r[i][1] == true {
				actual = r[i][0]
				count++
			}
		}
		assert.Equal(t, count, 2)
		for i := 0; i < len(r); i++ {
			assert.Equal(t, r[i][0], actual)
		}
	}
}
