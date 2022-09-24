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

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-spring/spring-base/knife"
)

type LoadType int

const (
	LoadNone  LoadType = iota // 获取失败
	LoadOnCtx                 // 从 context.Context 缓存获取
	LoadCache                 // 从本地缓存的值获取
	LoadBack                  // 从用户回调中获取
)

// TODO 可以增加分片功能，并发度较高时可以进一步提高性能。
var cache = &sync.Map{}

// InvalidateAll 删除所有缓存值，只用于单元测试。
func InvalidateAll() {
	cache = &sync.Map{}
}

type Result interface {
	Json() string
	Load(v interface{}) error
}

type valueResult struct {
	v reflect.Value
	t reflect.Type
}

func newValueResult(v interface{}) Result {
	return &valueResult{
		v: reflect.ValueOf(v),
		t: reflect.TypeOf(v),
	}
}

func (r *valueResult) Json() string {
	b, err := json.Marshal(r.v.Interface())
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func (r *valueResult) Load(v interface{}) error {
	outVal := reflect.ValueOf(v)
	if outVal.Kind() != reflect.Ptr || outVal.IsNil() {
		return errors.New("value should be ptr and not nil")
	}
	if outVal.Type().Elem() != r.t {
		return fmt.Errorf("type not match %s", outVal.Elem().Type())
	}
	outVal.Elem().Set(r.v)
	return nil
}

type jsonResult struct {
	v string
}

func (r *jsonResult) Json() string {
	return r.v
}

func (r *jsonResult) Load(v interface{}) error {
	return json.Unmarshal([]byte(r.v), v)
}

type ResultLoader func(ctx context.Context, key string) (Result, LoadType, error)

type cacheItem struct {
	value            Result
	writeTime        time.Time
	locker           sync.Mutex
	loader           ResultLoader
	expireAfterWrite time.Duration
	loading          *cacheItemLoading
}

type cacheItemLoading struct {
	v   Result
	err error
	wg  sync.WaitGroup
}

func (e *cacheItem) expired() bool {
	if e.expireAfterWrite <= 0 {
		return false
	}
	return time.Since(e.writeTime)-e.expireAfterWrite > 0
}

func (e *cacheItem) load(ctx context.Context, key string) (LoadType, Result, error) {

	e.locker.Lock()
	if p := e.loading; p != nil {
		e.locker.Unlock()
		p.wg.Wait()
		if p.err != nil {
			return LoadNone, nil, p.err
		}
		return LoadCache, p.v, nil
	}
	if e.value != nil && !e.expired() {
		e.locker.Unlock()
		return LoadCache, e.value, nil
	}
	c := &cacheItemLoading{}
	c.wg.Add(1)
	e.loading = c
	e.locker.Unlock()

	var loadType LoadType
	c.v, loadType, c.err = e.loader(ctx, key)
	c.wg.Done()

	e.locker.Lock()
	e.value = c.v
	e.writeTime = time.Now()
	e.loading = nil
	e.locker.Unlock()

	if c.err != nil {
		return LoadNone, nil, c.err
	}
	return loadType, c.v, nil
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
	return func(ctx context.Context, key string) (Result, LoadType, error) {
		//if replayer.ReplayMode() {
		//	resp, ok, err := replayer.Query(ctx, recorder.CACHE, key)
		//	if err != nil {
		//		return nil, LoadNone, err
		//	}
		//	if ok {
		//		return &jsonResult{v: resp}, LoadCache, nil
		//	}
		//}
		v, err := loader(ctx, key)
		if err != nil {
			return nil, LoadNone, err
		}
		return newValueResult(v), LoadBack, nil
	}
}

type optionArg struct {
	expireAfterWrite time.Duration
}

type Option func(*optionArg)

func ExpireAfterWrite(v time.Duration) Option {
	return func(arg *optionArg) {
		arg.expireAfterWrite = v
	}
}

func Load(ctx context.Context, key string, loader Loader, opts ...Option) (loadType LoadType, result Result, err error) {

	i, loaded, err := knife.LoadOrStore(ctx, key, newCtxItem())
	if err != nil {
		return LoadNone, nil, err
	}
	cItem := i.(*ctxItem)
	if loaded {
		cItem.wg.Wait()
		return LoadOnCtx, cItem.val, cItem.err
	}

	defer func() {
		cItem.val = result
		cItem.err = err
		cItem.wg.Done()
		if err != nil {
			knife.Delete(ctx, key)
		}
	}()

	arg := &optionArg{}
	for _, opt := range opts {
		opt(arg)
	}

	m := cache
	//if replayer.ReplayMode() {
	//	var v interface{}
	//	const ctxKey = "::CacheOnContext::"
	//	v, _, err = knife.LoadOrStore(ctx, ctxKey, &sync.Map{})
	//	if err != nil {
	//		return LoadNone, nil, err
	//	}
	//	m = v.(*sync.Map)
	//}
	//
	//defer func() {
	//	if loadType == LoadCache && recorder.RecordMode() {
	//		recorder.RecordAction(ctx, recorder.CACHE, &recorder.SimpleAction{
	//			Request: func() string {
	//				return key
	//			},
	//			Response: func() string {
	//				return result.Json()
	//			},
	//		})
	//	}
	//}()

	// TODO 检测 loader 是否发生变化，以便确认是否多个位置调用 Load 方法。
	actual, _ := m.LoadOrStore(key, &cacheItem{
		expireAfterWrite: arg.expireAfterWrite,
		loader:           toResultLoader(loader),
	})
	return actual.(*cacheItem).load(ctx, key)
}
