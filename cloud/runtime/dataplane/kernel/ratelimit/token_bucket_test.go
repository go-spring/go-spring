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

package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/stdlib/testing/assert"
)

func TestTokenBucketBurstAndRefill(t *testing.T) {
	b := ratelimit.NewTokenBucket(100, 2) // fast refill so the test never waits
	assert.That(t, b.AllowN(1)).True()
	assert.That(t, b.AllowN(1)).True()
	assert.That(t, b.AllowN(1)).False() // burst spent, refill not elapsed yet
	time.Sleep(20 * time.Millisecond)
	assert.That(t, b.AllowN(1)).True() // refilled
}

// TestTokenBucketWarmUpStartsCold pins the cold-start behavior: a warm-up
// bucket begins at burst/coldFactor tokens, not full burst.
func TestTokenBucketWarmUpStartsCold(t *testing.T) {
	b := ratelimit.NewTokenBucket(0.0001, 9).WithWarmUp(time.Hour, 3) // no refill within the test
	assert.That(t, b.AllowN(1)).True()
	assert.That(t, b.AllowN(1)).True()
	assert.That(t, b.AllowN(1)).True() // 3 = 9/3 cold tokens spent
	assert.That(t, b.AllowN(1)).False()
}

func TestTokenBucketWaitN(t *testing.T) {
	b := ratelimit.NewTokenBucket(100, 1)
	assert.That(t, b.AllowN(1)).True() // burst spent; the rest must wait on refill
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.That(t, b.WaitN(ctx, 1, time.Second)).True()        // waits for the refill
	assert.That(t, b.WaitN(ctx, 10, time.Millisecond)).False() // 10 never arrive in time
}

// TestTokenBucketUsageReadOnly pins the projection a state observer reads: it
// reports spent capacity without consuming or refilling anything.
func TestTokenBucketUsageReadOnly(t *testing.T) {
	b := ratelimit.NewTokenBucket(1, 10) // slow refill; reads must not perturb it

	usage := b.Usage()
	assert.That(t, usage.Limit).Equal(float64(10))
	assert.That(t, usage.Used < 0.01).True()

	assert.That(t, b.AllowN(4)).True()
	usage = b.Usage()
	assert.That(t, usage.Used > 3.99 && usage.Used < 4.01).True()

	// Reading spent nothing: the six remaining tokens are still available.
	assert.That(t, b.AllowN(6)).True()
}
