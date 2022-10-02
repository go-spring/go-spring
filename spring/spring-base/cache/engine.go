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
	"sync"
	"time"
)

type cacheItem struct {
	value            Result
	writeTime        time.Time
	locker           sync.Mutex
	loader           ResultLoader
	expireAfterWrite time.Duration
	loading          *cacheItemLoading
}

func (e *cacheItem) expired() bool {
	if e.expireAfterWrite <= 0 {
		return false
	}
	return time.Since(e.writeTime)-e.expireAfterWrite > 0
}

type cacheItemLoading struct {
	v   Result
	err error
	wg  sync.WaitGroup
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

	c.v, c.err = e.loader(ctx, key)
	c.wg.Done()

	e.locker.Lock()
	e.value = c.v
	e.writeTime = time.Now()
	e.loading = nil
	e.locker.Unlock()

	if c.err != nil {
		return LoadNone, nil, c.err
	}
	return LoadSource, c.v, nil
}

type engine struct{}

// Load loads value from m, if there is no cached value, call the loader
// to get value, and then stores it into m.
func (d *engine) Load(ctx context.Context, m *sync.Map, key string, loader ResultLoader, arg OptionArg) (loadType LoadType, result Result, err error) {
	actual, _ := m.LoadOrStore(key, &cacheItem{
		expireAfterWrite: arg.ExpireAfterWrite,
		loader:           loader,
	})
	return actual.(*cacheItem).load(ctx, key)
}
