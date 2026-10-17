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

	"go-spring.org/cloud/runtime/dataplane/ratelimit"
	"go-spring.org/stdlib/testing/assert"
)

func TestGCRABurstAndPace(t *testing.T) {
	g := ratelimit.NewGCRA(100, 2) // 10ms per event, 20ms burst tolerance
	assert.That(t, g.AllowN(1)).True()
	assert.That(t, g.AllowN(1)).True()
	assert.That(t, g.AllowN(1)).True()  // tolerance admits one more than the bucket
	assert.That(t, g.AllowN(1)).False() // schedule is beyond the tolerance
	time.Sleep(15 * time.Millisecond)
	assert.That(t, g.AllowN(1)).True() // the schedule has paced forward
}
