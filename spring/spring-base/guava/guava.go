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

var localCache sync.Map

func cache(ctx context.Context) (*sync.Map, error) {
	if replayer.ReplayMode() {
		key := "::CacheOnContext::"
		v, err := knife.LoadOrStore(ctx, key, &sync.Map{})
		if err != nil {
			return nil, err
		}
		return v.(*sync.Map), nil
	}
	return &localCache, nil
}

type Result interface {
	Load(v interface{}) error
	Json() string
}

type valueResult struct {
	value interface{}
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

func (r *valueResult) Json() string {
	return fastdev.ToJson(r.value)
}

type jsonResult struct {
	value string
}

func (r *jsonResult) Load(v interface{}) error {
	return json.Unmarshal([]byte(r.value), v)
}

func (r *jsonResult) Json() string {
	return r.value
}

type cacheItem struct {
	value  Result
	loader func(ctx context.Context, key string) (Result, error)
	locker sync.Mutex
	ttl    time.Duration
	start  time.Time
}

func (item *cacheItem) expired() bool {
	if item.ttl <= 0 {
		return false
	}
	return time.Since(item.start)-item.ttl > 0
}

func (item *cacheItem) Load(ctx context.Context, key string) (LoadType, Result, error) {

	item.locker.Lock()
	defer item.locker.Unlock()

	i, err := knife.Load(ctx, key)
	if err != nil {
		return LoadNone, nil, err
	}
	if i != nil {
		return LoadOnCtx, i.(Result), nil
	}

	defer func() {
		knife.Store(ctx, key, item.value)
	}()

	if item.value != nil && !item.expired() {
		return LoadCache, item.value, nil
	}

	v, err := item.loader(ctx, key)
	if err != nil {
		return LoadNone, nil, err
	}
	item.value = v
	item.start = time.Now()
	return LoadBack, item.value, nil
}

type Loader func(ctx context.Context, key string) (interface{}, error)

func Load(ctx context.Context, key string, ttl time.Duration, loader Loader) (loadType LoadType, result Result, _ error) {

	newLoader := func(ctx context.Context, key string) (Result, error) {
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

	m, err := cache(ctx)
	if err != nil {
		return LoadNone, nil, err
	}
	// TODO 检测 loader 是否发生变化，以便确认是否多个位置调用 Load 方法。
	actual, _ := m.LoadOrStore(key, &cacheItem{ttl: ttl, loader: newLoader})
	return actual.(*cacheItem).Load(ctx, key)
}
