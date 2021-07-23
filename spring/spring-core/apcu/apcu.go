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

// Package apcu 是进程内缓存，是 PHP APCu 的功能迁移。
package apcu

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"
)

var cache sync.Map

type cacheItem struct {
	source   interface{}
	expireAt time.Time
}

// OnLoad Load 成功获取到 key 对应的缓存值时发一个通知出来。
var OnLoad func(key string, val interface{})

// Load 获取 key 对应的缓存值，支持存入 string 但是按照 json 反序列化后的对象取出。
func Load(key string, out interface{}) (ok bool, err error) {

	v, ok := cache.Load(key)
	if !ok {
		return false, nil
	}

	item := v.(*cacheItem)
	if !item.expireAt.IsZero() && time.Now().After(item.expireAt) {
		return false, nil
	}

	outVal := reflect.ValueOf(out)
	if outVal.Kind() != reflect.Ptr || outVal.IsNil() {
		return false, &json.InvalidUnmarshalError{Type: outVal.Type()}
	}

	defer func() {
		if ok && OnLoad != nil {
			OnLoad(key, outVal.Interface())
		}
	}()

	if srcVal, ok := item.source.(reflect.Value); ok {
		if outVal.Type().Elem() == srcVal.Type() {
			outVal.Elem().Set(srcVal)
			return true, nil
		}
	}

	if str, ok := item.source.(string); ok {
		if err = json.Unmarshal([]byte(str), out); err != nil {
			return false, err
		}
		item.source = outVal.Elem()
		return true, nil
	}

	return false, fmt.Errorf("type not match %s", outVal.Type())
}

// Delete 删除 key 对应的缓存内容。
func Delete(key string) {
	cache.Delete(key)
}

type SaveArg struct {
	ttl time.Duration
}

type SaveOption func(arg *SaveArg)

// TTL 过期时间
func TTL(ttl time.Duration) SaveOption {
	return func(arg *SaveArg) {
		arg.ttl = ttl
	}
}

// Save 保存 key 及其对应的 val，支持对 key 设置 ttl 即过期时间。
func Save(key string, val interface{}, opts ...SaveOption) {
	arg := SaveArg{}
	for _, opt := range opts {
		opt(&arg)
	}
	expireAt := time.Time{}
	if arg.ttl > 0 {
		expireAt = time.Now().Add(arg.ttl)
	}
	cache.Store(key, &cacheItem{source: val, expireAt: expireAt})
}
