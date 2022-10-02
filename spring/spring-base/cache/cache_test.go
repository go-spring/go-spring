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

package cache_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cache"
)

type response struct {
	name string
}

type injection struct {
	expire time.Duration
	resp   interface{}
	data   *response
	err    error
}

var ctxInjectionKey int

func getInjection(ctx context.Context) *injection {
	return ctx.Value(&ctxInjectionKey).(*injection)
}

func setInjection(ctx context.Context, i *injection) context.Context {
	return context.WithValue(ctx, &ctxInjectionKey, i)
}

func loadResponse(ctx context.Context, key string, delay time.Duration) (*response, cache.LoadType, error) {

	i := getInjection(ctx)
	loader := func(ctx context.Context, key string) (interface{}, error) {
		if delay > 0 {
			time.Sleep(delay)
		}
		if i.err != nil {
			return nil, i.err
		}
		i.data.name = key
		return i.data, nil
	}

	opts := []cache.Option{
		cache.ExpireAfterWrite(i.expire),
	}
	loadType, result, err := cache.Load(ctx, key, loader, opts...)
	if err != nil {
		return nil, cache.LoadNone, err
	}

	if _, ok := i.resp.(*response); ok {
		var resp *response
		err = result.Load(&resp)
		if err != nil {
			return nil, cache.LoadNone, err
		}
		return resp, loadType, nil
	}

	var resp int
	err = result.Load(&resp)
	return nil, cache.LoadNone, err
}

type LoadTypeSlice []cache.LoadType

func (p LoadTypeSlice) Len() int           { return len(p) }
func (p LoadTypeSlice) Less(i, j int) bool { return p[i] > p[j] }
func (p LoadTypeSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func testFunc(key string, i *injection) ([]interface{}, []cache.LoadType) {
	ctx := setInjection(context.Background(), i)

	var (
		datas []interface{}
		types []cache.LoadType
		lock  sync.Mutex
	)

	wg := sync.WaitGroup{}
	for j := 0; j < 3; j++ {
		jj := j
		delay := 10 * time.Millisecond
		wg.Add(1)
		go func() {
			if jj == 1 {
				time.Sleep(delay)
			}
			defer wg.Done()
			resp, loadType, err := loadResponse(ctx, key, delay)
			lock.Lock()
			if err != nil {
				datas = append(datas, err)

			} else {
				datas = append(datas, resp)
			}
			types = append(types, loadType)
			lock.Unlock()
		}()
	}
	wg.Wait()

	sort.Sort(LoadTypeSlice(types))
	return datas, types
}

func TestCache(t *testing.T) {

	old := cache.Cache
	defer func() { cache.Cache = old }()

	for size := 1; size < 3; size++ {
		cache.Cache = cache.NewStorage(size, cache.SimpleHash)

		t.Run("response error", func(t *testing.T) {
			testKey := "test"
			defer func() {
				(cache.Cache).(*cache.Storage).Reset()
			}()
			i := &injection{
				err: errors.New("this is an error"),
			}
			datas, types := testFunc(testKey, i)
			assert.Equal(t, datas, []interface{}{
				errors.New("this is an error"),
				errors.New("this is an error"),
				errors.New("this is an error"),
			})
			assert.Equal(t, types, []cache.LoadType{
				cache.LoadNone,
				cache.LoadNone,
				cache.LoadNone,
			})
		})

		t.Run("response success", func(t *testing.T) {
			testKey := "test1234567890test1234567890"
			defer func() {
				(cache.Cache).(*cache.Storage).Reset()
			}()
			assert.False(t, cache.Has(testKey))
			i := &injection{
				expire: 50 * time.Millisecond,
				resp:   &response{},
				data:   &response{},
			}
			datas, types := testFunc(testKey, i)
			assert.Equal(t, datas, []interface{}{
				&response{name: testKey},
				&response{name: testKey},
				&response{name: testKey},
			})
			assert.Equal(t, types, []cache.LoadType{
				cache.LoadSource,
				cache.LoadCache,
				cache.LoadCache,
			})
			time.Sleep(150 * time.Millisecond)
			assert.False(t, cache.Has(testKey))
		})

		t.Run("response success without expired", func(t *testing.T) {
			testKey := "test1234567890"
			defer func() {
				(cache.Cache).(*cache.Storage).Reset()
			}()
			assert.False(t, cache.Has(testKey))
			i := &injection{
				resp: &response{},
				data: &response{},
			}
			datas, types := testFunc(testKey, i)
			assert.Equal(t, datas, []interface{}{
				&response{name: testKey},
				&response{name: testKey},
				&response{name: testKey},
			})
			assert.Equal(t, types, []cache.LoadType{
				cache.LoadSource,
				cache.LoadCache,
				cache.LoadCache,
			})
			time.Sleep(150 * time.Millisecond)
			assert.True(t, cache.Has(testKey))
		})

		t.Run("load error", func(t *testing.T) {
			testKey := "test"
			defer func() {
				(cache.Cache).(*cache.Storage).Reset()
			}()
			i := &injection{
				resp: map[string]string{},
				data: &response{},
			}
			datas, types := testFunc(testKey, i)
			assert.Equal(t, datas, []interface{}{
				errors.New("load type (int) but expect type (*cache_test.response)"),
				errors.New("load type (int) but expect type (*cache_test.response)"),
				errors.New("load type (int) but expect type (*cache_test.response)"),
			})
			assert.Equal(t, types, []cache.LoadType{
				cache.LoadNone,
				cache.LoadNone,
				cache.LoadNone,
			})
		})
	}
}
