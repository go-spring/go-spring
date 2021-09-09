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

package internal

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-spring/spring-boost/json"
)

type cacheItem struct {
	source   interface{}
	expireAt time.Time
}

type cache struct {
	m sync.Map
}

// New 返回一个 APCU 的实例。
func New() *cache {
	return &cache{}
}

func (c *cache) Load(_ context.Context, key string, out interface{}) (ok bool, err error) {

	v, ok := c.m.Load(key)
	if !ok {
		return false, nil
	}

	item := v.(*cacheItem)
	if item.source == nil {
		return false, nil
	}

	// 缓存过期之后不会删除 key 对应的 *cacheItem 对象，而是将 *cacheItem
	// 对象的 source 设为 nil，原因是此时此处的 Delete 操作无法和此时别处的
	// Store 操作保持顺序，即此处检测到了过期，但是别处正在执行 Store 操作，
	// 那么此处的 Delete 理应在 Store 之前执行，但是目前的框架下是无法保证的。
	// 因此，退而求其次，把缓存的真实内容释放掉，这样即使占了一些内存也不会太多。
	if !item.expireAt.IsZero() && time.Now().After(item.expireAt) {
		c.m.Store(key, &cacheItem{expireAt: item.expireAt})
		return false, nil
	}

	outVal := reflect.ValueOf(out)
	if outVal.Kind() != reflect.Ptr || outVal.IsNil() {
		return false, errors.New("out value should be ptr and not nil")
	}

	switch source := item.source.(type) {
	case reflect.Value:
		if outVal.Type().Elem() == source.Type() {
			outVal.Elem().Set(source)
			return true, nil
		}
	case string:
		err = json.Unmarshal([]byte(source), out)
		if err != nil {
			return false, err
		}
		item.source = outVal.Elem()
		return true, nil
	default:
		srcVal := reflect.ValueOf(source)
		if srcVal.Type() == outVal.Type().Elem() {
			outVal.Elem().Set(srcVal)
			return true, nil
		}
	}
	return false, fmt.Errorf("type not match %s", outVal.Elem().Type())
}

type StoreArg struct {
	TTL time.Duration
}

type StoreOption func(arg *StoreArg)

func (c *cache) Store(key string, val interface{}, opts ...StoreOption) {
	arg := StoreArg{}
	for _, opt := range opts {
		opt(&arg)
	}
	expireAt := time.Time{}
	if arg.TTL > 0 {
		expireAt = time.Now().Add(arg.TTL)
	}
	c.m.Store(key, &cacheItem{source: val, expireAt: expireAt})
}

func (c *cache) Range(f func(key, value interface{}) bool) {
	c.m.Range(f)
}

func (c *cache) Delete(key string) {
	c.m.Delete(key)
}
