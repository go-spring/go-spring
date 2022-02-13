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

package local_test

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/caching/ctxon"
	"github.com/go-spring/spring-base/caching/local"
)

type LoadTypeSlice []local.LoadType

func (p LoadTypeSlice) Len() int           { return len(p) }
func (p LoadTypeSlice) Less(i, j int) bool { return p[i] > p[j] }
func (p LoadTypeSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func runCase(t *testing.T, load func(ctx context.Context) local.LoadType, expectLoadTypes []local.LoadType) {

	var (
		locker    sync.Mutex
		waitGroup sync.WaitGroup
		loadTypes []local.LoadType
	)

	ctx, cached := ctxon.New(context.Background())
	assert.False(t, cached)

	for i := 0; i < 3; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			loadType := load(ctx)
			locker.Lock()
			defer locker.Unlock()
			loadTypes = append(loadTypes, loadType)
		}()
	}
	waitGroup.Wait()

	sort.Sort(LoadTypeSlice(loadTypes))
	assert.Equal(t, loadTypes, expectLoadTypes)
}

func TestCaching(t *testing.T) {

	t.Run("int", func(t *testing.T) {
		t.Parallel()

		var key = "int"
		err := local.Store(context.Background(), key, 5)
		assert.Nil(t, err)

		load := func(ctx context.Context) local.LoadType {
			var (
				loadVal  int
				loadType local.LoadType
			)
			loadType, err = local.Load(ctx, key, &loadVal)
			assert.Nil(t, err)
			assert.Equal(t, loadVal, 5)
			return loadType
		}

		runCase(t, load, []local.LoadType{local.LoadCache, local.LoadOnCtx, local.LoadOnCtx})
		runCase(t, load, []local.LoadType{local.LoadCache, local.LoadOnCtx, local.LoadOnCtx})
	})

	t.Run("lazy_int", func(t *testing.T) {
		t.Parallel()

		var key = "lazy_int"
		err := local.StoreTTL(context.Background(), key, 500*time.Millisecond,
			func(ctx context.Context, key string) (interface{}, error) {
				return 5, nil
			})
		assert.Nil(t, err)

		load := func(ctx context.Context) local.LoadType {
			var (
				loadVal  int
				loadType local.LoadType
			)
			loadType, err = local.Load(ctx, key, &loadVal)
			assert.Nil(t, err)
			assert.Equal(t, loadVal, 5)
			return loadType
		}

		runCase(t, load, []local.LoadType{local.LoadBack, local.LoadOnCtx, local.LoadOnCtx})
		runCase(t, load, []local.LoadType{local.LoadCache, local.LoadOnCtx, local.LoadOnCtx})
		time.Sleep(500 * time.Millisecond)
		runCase(t, load, []local.LoadType{local.LoadBack, local.LoadOnCtx, local.LoadOnCtx})

		err = local.StoreTTL(context.Background(), key, 500*time.Millisecond,
			func(ctx context.Context, key string) (interface{}, error) {
				return "5", nil
			})
		assert.Nil(t, err)

		runCase(t, load, []local.LoadType{local.LoadBack, local.LoadOnCtx, local.LoadOnCtx})
		runCase(t, load, []local.LoadType{local.LoadCache, local.LoadOnCtx, local.LoadOnCtx})
		time.Sleep(500 * time.Millisecond)
		runCase(t, load, []local.LoadType{local.LoadBack, local.LoadOnCtx, local.LoadOnCtx})
	})

	t.Run("struct", func(t *testing.T) {
		t.Parallel()

		type StructT struct {
			A string `json:"a"`
		}

		var key = "struct"
		err := local.Store(context.Background(), key, StructT{A: "x"})
		assert.Nil(t, err)

		load := func(ctx context.Context) local.LoadType {
			var (
				loadVal  StructT
				loadType local.LoadType
			)
			loadType, err = local.Load(ctx, key, &loadVal)
			assert.Nil(t, err)
			assert.Equal(t, loadVal, StructT{A: "x"})
			return loadType
		}

		runCase(t, load, []local.LoadType{local.LoadCache, local.LoadOnCtx, local.LoadOnCtx})
		runCase(t, load, []local.LoadType{local.LoadCache, local.LoadOnCtx, local.LoadOnCtx})
	})

	t.Run("lazy_struct", func(t *testing.T) {
		t.Parallel()

		type StructT struct {
			A string `json:"a"`
		}

		var key = "lazy_struct"
		err := local.StoreTTL(context.Background(), key, 500*time.Millisecond,
			func(ctx context.Context, key string) (interface{}, error) {
				return StructT{A: "x"}, nil
			})
		assert.Nil(t, err)

		load := func(ctx context.Context) local.LoadType {
			var (
				loadVal  StructT
				loadType local.LoadType
			)
			loadType, err = local.Load(ctx, key, &loadVal)
			assert.Nil(t, err)
			assert.Equal(t, loadVal, StructT{A: "x"})
			return loadType
		}

		runCase(t, load, []local.LoadType{local.LoadBack, local.LoadOnCtx, local.LoadOnCtx})
		runCase(t, load, []local.LoadType{local.LoadCache, local.LoadOnCtx, local.LoadOnCtx})
		time.Sleep(500 * time.Millisecond)
		runCase(t, load, []local.LoadType{local.LoadBack, local.LoadOnCtx, local.LoadOnCtx})

		err = local.StoreTTL(context.Background(), key, 500*time.Millisecond,
			func(ctx context.Context, key string) (interface{}, error) {
				return "{\"a\":\"x\"}", nil
			})
		assert.Nil(t, err)

		load = func(ctx context.Context) local.LoadType {
			var (
				loadVal  *StructT
				loadType local.LoadType
			)
			loadType, err = local.Load(ctx, key, &loadVal)
			assert.Nil(t, err)
			assert.Equal(t, loadVal, &StructT{A: "x"})
			return loadType
		}

		runCase(t, load, []local.LoadType{local.LoadBack, local.LoadOnCtx, local.LoadOnCtx})
		runCase(t, load, []local.LoadType{local.LoadCache, local.LoadOnCtx, local.LoadOnCtx})
		time.Sleep(500 * time.Millisecond)
		runCase(t, load, []local.LoadType{local.LoadBack, local.LoadOnCtx, local.LoadOnCtx})

	})
}
