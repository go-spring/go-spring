/*
 * Copyright 2025 The Go-Spring Authors.
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

package ctxcache

import (
	"fmt"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestCtxCache(t *testing.T) {

	// Operations on uninitialized context
	err := Set(t.Context(), "key", "value")
	fmt.Println(err)
	assert.Error(t, err).Is(ErrCacheNotInitialized)

	_, err = Get[string](t.Context(), "key")
	fmt.Println(err)
	assert.Error(t, err).Is(ErrCacheNotInitialized)

	// Check repeated Init
	ctx1, cancel1 := Init(t.Context())
	ctx2, cancel2 := Init(ctx1)
	assert.That(t, ctx1).Same(ctx2)

	// Getting an unset key should fail
	_, err = Get[string](ctx1, "key")
	fmt.Println(err)
	assert.Error(t, err).Is(ErrKeyNotSet)

	// Set and Get a string value successfully
	err = Set(ctx1, "key", "value")
	fmt.Println(err)
	assert.Error(t, err).Nil()

	value, err := Get[string](ctx1, "key")
	fmt.Println(err)
	assert.Error(t, err).Nil()
	assert.String(t, value).Equal("value")

	// Setting the same key twice should fail
	err = Set(ctx1, "key", "anotherValue")
	fmt.Println(err)
	assert.Error(t, err).Is(ErrKeyAlreadySet)

	// Set and Get multiple types under same key
	err = Set(ctx1, "key", 42)
	fmt.Println(err)
	assert.Error(t, err).Nil()

	intValue, err := Get[int](ctx1, "key")
	fmt.Println(err)
	assert.Error(t, err).Nil()
	assert.Number(t, intValue).Equal(42)

	// Test cache clearing via cancel
	cancel1()

	_, err = Get[string](ctx1, "key")
	fmt.Println(err)
	assert.Error(t, err).Is(ErrCacheAlreadyCleared)

	// Re-calling cancel should be safe
	cancel2()

	err = Set(ctx1, "key", "value")
	fmt.Println(err)
	assert.Error(t, err).Is(ErrCacheAlreadyCleared)

	_, err = Get[int](ctx1, "key")
	fmt.Println(err)
	assert.Error(t, err).Is(ErrCacheAlreadyCleared)
}
