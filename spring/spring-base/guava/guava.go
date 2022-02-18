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

package guava

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/recorder"
	"github.com/go-spring/spring-base/fastdev/replayer"
	"github.com/go-spring/spring-base/knife"
)

type LoadType int

const (
	LoadNone  LoadType = iota // 获取失败
	LoadOnCtx                 // 从 context.Context 缓存获取
	LoadCache                 // 从本地缓存的值获取
	LoadBack                  // 从用户回调中获取
)

var cache = &sync.Map{}

func InvalidateAll() {
	cache = &sync.Map{}
}

type Result interface {
	Load(v interface{}) error
	Json() string
}

type valueResult struct {
	value interface{}
}

func (r *valueResult) Json() string {
	return fastdev.ToJson(r.value)
}

func (r *valueResult) Load(v interface{}) error {
	outVal := reflect.ValueOf(v)
	if outVal.Kind() != reflect.Ptr || outVal.IsNil() {
		return errors.New("value should be ptr and not nil")
	}
	srcVal := reflect.ValueOf(r.value)
	outVal.Elem().Set(srcVal)
	return nil
}

type jsonResult struct {
	value string
}

func (r *jsonResult) Json() string {
	return r.value
}

func (r *jsonResult) Load(v interface{}) error {
	return json.Unmarshal([]byte(r.value), v)
}

type ResultLoader func(ctx context.Context, key string) (Result, error)

type cacheItem struct {
	value  Result
	loader ResultLoader
	locker sync.RWMutex

	expireAfterWrite time.Duration
	writeTime        time.Time
}

func (item *cacheItem) expired() bool {
	if item.expireAfterWrite <= 0 {
		return false
	}
	return time.Since(item.writeTime)-item.expireAfterWrite > 0
}

func (item *cacheItem) Load(ctx context.Context, key string) (LoadType, Result, error) {
	loadType, result := item.fastLoad()
	if loadType != LoadNone {
		return loadType, result, nil
	}
	return item.slowLoad(ctx, key)
}

func (item *cacheItem) fastLoad() (LoadType, Result) {
	item.locker.RLock()
	defer item.locker.RUnlock()
	if item.value != nil && !item.expired() {
		return LoadCache, item.value
	}
	return LoadNone, nil
}

func (item *cacheItem) slowLoad(ctx context.Context, key string) (LoadType, Result, error) {
	item.locker.Lock()
	defer item.locker.Unlock()
	if item.value != nil && !item.expired() {
		return LoadCache, item.value, nil
	}
	v, err := item.loader(ctx, key)
	if err != nil {
		return LoadNone, nil, err
	}
	item.value = v
	item.writeTime = time.Now()
	return LoadBack, item.value, nil
}

type ctxItem struct {
	val Result
	err error
	wg  sync.WaitGroup
}

func newCtxItem() *ctxItem {
	c := &ctxItem{}
	c.wg.Add(1)
	return c
}

type Loader func(ctx context.Context, key string) (interface{}, error)

func toResultLoader(loader Loader) ResultLoader {
	return func(ctx context.Context, key string) (Result, error) {
		if replayer.ReplayMode() {
			resp, err := replayer.QueryAction(ctx, fastdev.APCU, key, replayer.ExactMatch)
			if err != nil {
				return nil, err
			}
			if resp != nil {
				return &jsonResult{value: resp.(string)}, nil
			}
		}
		i, err := loader(ctx, key)
		if err != nil {
			return nil, err
		}
		return &valueResult{value: i}, nil
	}
}

func GetOrLoad(ctx context.Context, key string, expireAfterWrite time.Duration, loader Loader) (loadType LoadType, result Result, err error) {

	i, loaded, err := knife.LoadOrStore(ctx, key, newCtxItem())
	if err != nil {
		return LoadNone, nil, err
	}
	cItem := i.(*ctxItem)
	if loaded {
		if cItem.val != nil {
			return LoadOnCtx, cItem.val, nil
		}
		cItem.wg.Wait()
		return LoadOnCtx, cItem.val, cItem.err
	}

	defer func() {
		cItem.val = result
		cItem.err = err
		cItem.wg.Done()
	}()

	m := cache
	if replayer.ReplayMode() {
		var v interface{}
		const ctxKey = "::CacheOnContext::"
		v, _, err = knife.LoadOrStore(ctx, ctxKey, &sync.Map{})
		if err != nil {
			return LoadNone, nil, err
		}
		m = v.(*sync.Map)
	}

	defer func() {
		if loadType == LoadCache && recorder.EnableRecord(ctx) {
			recorder.RecordAction(ctx, &fastdev.Action{
				Protocol: fastdev.APCU,
				Request: fastdev.NewMessage(func() string {
					return key
				}),
				Response: fastdev.NewMessage(func() string {
					return result.Json()
				}),
			})
		}
	}()

	// TODO 检测 loader 是否发生变化，以便确认是否多个位置调用 Load 方法。
	actual, _ := m.LoadOrStore(key, &cacheItem{expireAfterWrite: expireAfterWrite, loader: toResultLoader(loader)})
	return actual.(*cacheItem).Load(ctx, key)
}
