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
	"testing"
	"time"
)

// TestDefaultDriverBuildsAWorkingCache pins that the bundled assembly maps the
// configuration onto bigcache's own and returns a cache that is already wired:
// [NewCache] is applied inside, so the driver's product is complete and usable
// without further steps.
func TestDefaultDriverBuildsAWorkingCache(t *testing.T) {
	c, err := DefaultDriver{}.CreateClient(context.Background(), "hot", Config{
		Shards:             16,
		LifeWindow:         time.Minute,
		CleanWindow:        time.Minute,
		MaxEntriesInWindow: 100,
		MaxEntrySize:       128,
	})
	if err != nil {
		t.Fatalf("CreateClient: %v", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.Set(context.Background(), "k", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if b, err := c.Get(context.Background(), "k"); err != nil || string(b) != "v" {
		t.Fatalf("Get = %q, %v; want \"v\", nil", b, err)
	}
}

// TestDefaultDriverSurfacesBuildErrors pins that a misconfiguration fails at
// assembly instead of being papered over: bigcache rejects a shard count that is
// not a power of two, and the driver returns that rejection unchanged so it
// surfaces at boot.
func TestDefaultDriverSurfacesBuildErrors(t *testing.T) {
	if _, err := (DefaultDriver{}).CreateClient(context.Background(), "bad", Config{Shards: 3}); err == nil {
		t.Fatal("CreateClient with 3 shards: want an error, got nil")
	}
}
