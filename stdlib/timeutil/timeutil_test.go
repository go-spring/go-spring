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

package timeutil_test

import (
	"context"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/timeutil"
)

func TestSleep(t *testing.T) {
	// Full duration elapses: true.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := time.Now()
	assert.That(t, timeutil.Sleep(ctx, 10*time.Millisecond)).True()
	assert.That(t, time.Since(start) >= 10*time.Millisecond).True()

	// Cancelled mid-sleep: false, promptly.
	cancel()
	assert.That(t, timeutil.Sleep(ctx, time.Second)).False()

	// Non-positive duration: immediate true on a live ctx.
	fresh, freshCancel := context.WithCancel(context.Background())
	defer freshCancel()
	assert.That(t, timeutil.Sleep(fresh, 0)).True()
}
