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

package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-spring/spring-base/caching/ctxon"
)

type LoadType int

const (
	LoadNone  LoadType = iota // 获取失败
	LoadOnCtx                 // 从 context.Context 缓存获取
	LoadCache                 // 从本地缓存的值获取
	LoadBack                  // 本地没有或者过期时回源进行获取
)

var cache sync.Map

func GetKey(ctx context.Context, key string) (string, error) {
	//if replayer.ReplayMode() {
	//	sessionID, err := replayer.GetSessionID(ctx)
	//	if err != nil {
	//		return "", err
	//	}
	//	return sessionID + "." + key, nil
	//}
	return key, nil
}

type loader interface {
	expired() bool
	load(ctx context.Context, key string) (v interface{}, cached bool, err error)
}

type cacheValue struct {
	v reflect.Value
	t reflect.Type
}

type cacheItem struct {
	loader loader
	value  *cacheValue
	locker sync.Mutex
}

func Load(ctx context.Context, key string, out interface{}) (loadType LoadType, err error) {
	defer func() {
		if loadType == LoadCache {
			// 只有从本地缓存里面获取值得时候才需要录制。
		}
	}()

	return load(ctx, key, out)
}

func load(ctx context.Context, key string, out interface{}) (LoadType, error) {

	outVal := reflect.ValueOf(out)
	if outVal.Kind() != reflect.Ptr || outVal.IsNil() {
		return LoadNone, errors.New("out value should be ptr and not nil")
	}

	loadType, err := loadFromCtx(ctx, key, outVal)
	if err != nil || loadType != LoadNone {
		return loadType, err
	}

	loadKey, err := GetKey(ctx, key)
	if err != nil {
		return LoadNone, err
	}

	v, ok := cache.Load(loadKey)
	if !ok {
		return LoadNone, errors.New("no key stored")
	}

	item := v.(*cacheItem)
	item.locker.Lock()
	defer item.locker.Unlock()

	loadType, err = loadFromCtx(ctx, key, outVal)
	if err != nil || loadType != LoadNone {
		return loadType, err
	}

	var (
		fromCache bool
		itemValue reflect.Value
	)

	if item.value != nil {
		fromCache = true
		itemValue = item.value.v
	}

	if item.value == nil || item.loader.expired() {
		var value interface{}
		value, fromCache, err = item.loader.load(ctx, key)
		if err != nil {
			return LoadNone, err
		}
		switch source := value.(type) {
		case string:
			outType := reflect.TypeOf(out)
			val := reflect.New(outType.Elem())
			err = json.Unmarshal([]byte(source), val.Interface())
			if err != nil {
				if outVal.Elem().Kind() != reflect.String {
					return LoadNone, err
				}
				itemValue = reflect.ValueOf(value)
			} else {
				itemValue = val.Elem()
			}
		default:
			itemValue = reflect.ValueOf(value)
		}
		item.value = &cacheValue{
			t: itemValue.Type(),
			v: itemValue,
		}
	}

	if outVal.Type().Elem() != item.value.t {
		return LoadNone, fmt.Errorf("type not match %s", outVal.Elem().Type())
	}

	if err = ctxon.Set(ctx, key, item.value); err != nil {
		return LoadNone, err
	}
	outVal.Elem().Set(itemValue)
	if fromCache {
		return LoadCache, nil
	}
	return LoadBack, nil
}

func loadFromCtx(ctx context.Context, key string, outVal reflect.Value) (LoadType, error) {
	i, ok := ctxon.Get(ctx, key)
	if !ok {
		return LoadNone, nil
	}
	c := i.(*cacheValue)
	if outVal.Type().Elem() != c.t {
		return LoadNone, fmt.Errorf("type not match %s", outVal.Elem().Type())
	}
	outVal.Elem().Set(c.v)
	return LoadOnCtx, nil
}

type valueLoader struct {
	value interface{}
}

func (loader *valueLoader) expired() bool {
	return false
}

func (loader *valueLoader) load(ctx context.Context, key string) (v interface{}, cached bool, err error) {
	return loader.value, true, nil
}

func Store(ctx context.Context, key string, value interface{}) error {
	key, err := GetKey(ctx, key)
	if err != nil {
		return err
	}
	l := &valueLoader{value: value}
	cache.Store(key, &cacheItem{loader: l})
	return nil
}

type Loading func(ctx context.Context, key string) (interface{}, error)

type lazyLoader struct {
	loading Loading
	locker  sync.Mutex
	ttl     time.Duration
	start   time.Time
	value   interface{}
}

func (l *lazyLoader) expired() bool {
	return time.Since(l.start)-l.ttl >= 0
}

func (l *lazyLoader) load(ctx context.Context, key string) (v interface{}, cached bool, err error) {
	if !l.expired() {
		return l.value, true, nil
	}
	l.locker.Lock()
	defer l.locker.Unlock()
	if !l.expired() {
		return l.value, true, nil
	}
	v, err = l.loading(ctx, key)
	if err != nil {
		return "", false, err
	}
	l.value = v
	l.start = time.Now()
	return l.value, false, err
}

func StoreTTL(ctx context.Context, key string, ttl time.Duration, loading Loading) error {
	if ttl <= 0 {
		return errors.New("invalid ttl")
	}
	key, err := GetKey(ctx, key)
	if err != nil {
		return err
	}
	l := &lazyLoader{loading: loading, ttl: ttl}
	cache.Store(key, &cacheItem{loader: l})
	return nil
}

// Delete 删除 key 对应的缓存内容。
func Delete(ctx context.Context, key string) error {
	key, err := GetKey(ctx, key)
	if err != nil {
		return err
	}
	cache.Delete(key)
	return nil
}

// Range 遍历缓存的内容。
func Range(f func(key, value interface{}) bool) {
	cache.Range(func(key, value interface{}) bool {
		itemValue := value.(*cacheItem).value
		return f(key, itemValue.v.Interface())
	})
}
