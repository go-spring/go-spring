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
	"context"
	"time"

	"github.com/go-spring/spring-boost/apcu/internal"
	"github.com/go-spring/spring-boost/recorder"
	"github.com/go-spring/spring-boost/replayer"
)

const Protocol = "apcu"

var cache internal.APCU = internal.New()

// Load 获取 key 对应的缓存值，注意 out 的类型必须和 Store 的时候存入的类
// 型一致，否则 Load 会失败。但是如果 Store 的时候存入的内容是一个字符串，
// 那么 out 可以是该字符串 JSON 反序列化之后的类型。
func Load(ctx context.Context, key string, out interface{}) (ok bool, err error) {

	if replayer.ReplayMode() {
		return replayer.Replay(ctx, &recorder.Action{
			Protocol: Protocol,
			Key:      key,
			Output:   out,
		})
	}

	defer func() {
		if ok {
			recorder.Record(ctx, func() *recorder.Action {
				return &recorder.Action{
					Protocol: Protocol,
					Key:      key,
					Output:   out,
				}
			})
		}
	}()

	return cache.Load(ctx, key, out)
}

type StoreOption = internal.StoreOption

// TTL 设置 key 的过期时间。
func TTL(ttl time.Duration) StoreOption {
	return func(arg *internal.StoreArg) {
		arg.TTL = ttl
	}
}

// Store 保存 key 及其对应的 val，支持对 key 设置 ttl (过期时间)。另外，
// 这里的 val 可以是任何值，因此要求 Load 的时候必须保证返回值和这里的 val
// 是相同类型的，否则 Load 会失败。
// 但是这里有一个例外情况，考虑到很多场景下，用户需要缓存一个由字符串反序列
// 化后的对象，所以该库提供了一个功能，就是用户可以 Store 一个字符串，然后
// Load 的时候按照指定类型返回。
func Store(key string, val interface{}, opts ...StoreOption) {
	cache.Store(key, val, opts...)
}

// Range 遍历缓存的内容。
func Range(f func(key, value interface{}) bool) {
	cache.Range(f)
}

// Delete 删除 key 对应的缓存内容。
func Delete(key string) {
	cache.Delete(key)
}
