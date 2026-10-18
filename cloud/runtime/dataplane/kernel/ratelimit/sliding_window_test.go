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
	"testing"
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/stdlib/testing/assert"
)

func TestSlidingWindowLimit(t *testing.T) {
	w := ratelimit.NewSlidingWindow(2, time.Hour) // never rolls within the test
	assert.That(t, w.AllowN(1)).True()
	assert.That(t, w.AllowN(1)).True()
	assert.That(t, w.AllowN(1)).False()
}
