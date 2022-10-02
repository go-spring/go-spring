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

// Package cache provides object caching in memory.
package cache

import (
	"context"
	"sync"
	"time"
)

var (
	Cache  Shardable
	Engine Driver
)

func init() {
	Cache = NewStorage(1, SimpleHash)
	Engine = &engine{}
}

// A Shardable cache can improve the performance of concurrency.
type Shardable interface {
	Sharding(key string) *sync.Map
}

type LoadType int

const (
	LoadNone LoadType = iota
	LoadOnCtx
	LoadCache
	LoadSource
)

// ResultLoader returns a wrapper for the source value of the key.
type ResultLoader func(ctx context.Context, key string) (Result, error)

// Driver loads value from m, if there is no cached value, call the loader
// to get value, and then stores it into m.
type Driver interface {
	Load(ctx context.Context, m *sync.Map, key string, loader ResultLoader, arg OptionArg) (loadType LoadType, result Result, err error)
}

// Has returns whether the key exists.
func Has(key string) bool {
	v, ok := Cache.Sharding(key).Load(key)
	if ok && v != nil {
		return !v.(*cacheItem).expired()
	}
	return false
}

type OptionArg struct {
	ExpireAfterWrite time.Duration
}

type Option func(*OptionArg)

// ExpireAfterWrite sets the expiration time of the cache value after write.
func ExpireAfterWrite(v time.Duration) Option {
	return func(arg *OptionArg) {
		arg.ExpireAfterWrite = v
	}
}

// Loader gets value from a background, such as Redis, MySQL, etc.
type Loader func(ctx context.Context, key string) (interface{}, error)

// Load loads value from cache, if there is no cached value, call the loader
// to get value, and then stores it.
func Load(ctx context.Context, key string, loader Loader, opts ...Option) (loadType LoadType, result Result, _ error) {
	arg := OptionArg{}
	for _, opt := range opts {
		opt(&arg)
	}
	l := func(ctx context.Context, key string) (Result, error) {
		v, err := loader(ctx, key)
		if err != nil {
			return nil, err
		}
		return NewValueResult(v), nil
	}
	m := Cache.Sharding(key)
	return Engine.Load(ctx, m, key, l, arg)
}
