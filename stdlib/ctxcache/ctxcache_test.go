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
	"errors"
	"fmt"
	"testing"
)

func TestCtxCache(t *testing.T) {

	// Operations on uninitialized context
	err := Set(t.Context(), "key", "value")
	fmt.Println(err)
	if err == nil || !errors.Is(err, ErrCacheNotInitialized) {
		t.Error("Expected ErrCacheNotInitialized when calling Set on unbound context")
	}

	_, err = Get[string](t.Context(), "key")
	fmt.Println(err)
	if err == nil || !errors.Is(err, ErrCacheNotInitialized) {
		t.Error("Expected ErrCacheNotInitialized when calling Get on unbound context")
	}

	// Check repeated Init
	ctx1, cancel1 := Init(t.Context())
	ctx2, cancel2 := Init(ctx1)
	if ctx1 != ctx2 {
		t.Error("Expected repeated Init to return the same context")
	}

	// Getting an unset key should fail
	_, err = Get[string](ctx1, "key")
	fmt.Println(err)
	if err == nil || !errors.Is(err, ErrKeyNotSet) {
		t.Error("Expected ErrKeyNotSet for unset key")
	}

	// Set and Get a string value successfully
	err = Set(ctx1, "key", "value")
	fmt.Println(err)
	if err != nil {
		t.Fatalf("Set string failed: %v", err)
	}

	value, err := Get[string](ctx1, "key")
	fmt.Println(err)
	if err != nil {
		t.Fatalf("Get string failed: %v", err)
	}
	if value != "value" {
		t.Errorf("Expected 'value', got '%s'", value)
	}

	// Setting the same key twice should fail
	err = Set(ctx1, "key", "anotherValue")
	fmt.Println(err)
	if err == nil || !errors.Is(err, ErrKeyAlreadySet) {
		t.Error("Expected ErrKeyAlreadySet when setting the same key twice")
	}

	// Set and Get multiple types under same key
	err = Set(ctx1, "key", 42)
	fmt.Println(err)
	if err != nil {
		t.Fatalf("Set int failed: %v", err)
	}

	intValue, err := Get[int](ctx1, "key")
	fmt.Println(err)
	if err != nil {
		t.Fatalf("Get int failed: %v", err)
	}
	if intValue != 42 {
		t.Errorf("Expected 42, got %d", intValue)
	}

	// Test cache clearing via cancel
	cancel1()

	_, err = Get[string](ctx1, "key")
	fmt.Println(err)
	if err == nil || !errors.Is(err, ErrCacheAlreadyCleared) {
		t.Error("Expected ErrCacheAlreadyCleared after cancel")
	}

	// Re-calling cancel should be safe
	cancel2()

	err = Set(ctx1, "key", "value")
	fmt.Println(err)
	if err == nil || !errors.Is(err, ErrCacheAlreadyCleared) {
		t.Error("Expected ErrCacheAlreadyCleared when setting after cancel")
	}

	_, err = Get[int](ctx1, "key")
	fmt.Println(err)
	if err == nil || !errors.Is(err, ErrCacheAlreadyCleared) {
		t.Error("Expected ErrCacheAlreadyCleared after cancel")
	}
}
