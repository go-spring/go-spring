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

package StarterBigCache

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/cache"
)

// TestByteCachePrimitives pins the translation this adapter exists for: the
// typed façade above it branches on [cache.ErrMiss], while bigcache reports a
// missing key as its own sentinel. Deleting an absent key is the mirror case —
// an error below, a no-op above.
func TestByteCachePrimitives(t *testing.T) {
	c := newTestCache(t, "hot") // holds k=v
	defer func() { _ = c.Destroy() }()

	bc := NewByteCache(c)
	ctx := context.Background()

	b, err := bc.GetBytes(ctx, "k")
	if err != nil || string(b) != "v" {
		t.Fatalf("GetBytes(k) = %q, %v; want \"v\", nil", b, err)
	}

	if _, err := bc.GetBytes(ctx, "absent"); !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("GetBytes(absent) = %v, want cache.ErrMiss", err)
	}

	// SetBytes ignores its ttl argument: bigcache expires an entry by the
	// instance's LifeWindow alone, and the adapter documents that rather than
	// pretending to honour it.
	if err := bc.SetBytes(ctx, "other", []byte("v2"), 3600); err != nil {
		t.Fatalf("SetBytes: %v", err)
	}
	if b, err := bc.GetBytes(ctx, "other"); err != nil || string(b) != "v2" {
		t.Fatalf("GetBytes(other) = %q, %v; want \"v2\", nil", b, err)
	}

	if err := bc.Delete(ctx, "absent"); err != nil {
		t.Fatalf("Delete(absent) = %v, want nil", err)
	}
	if err := bc.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete(k) = %v", err)
	}
	if _, err := bc.GetBytes(ctx, "k"); !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("GetBytes(k) after Delete = %v, want cache.ErrMiss", err)
	}
}
