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

package testcases

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/redis"
)

var Append = func(t *testing.T, ctx context.Context, c redis.Client) {

	exists, err := c.Exists(ctx, "mykey")
	if err != nil {
		t.Fatal()
	}
	assert.False(t, exists)

	count, err := c.Append(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, count, int64(5))

	count, err = c.Append(ctx, "mykey", " World")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, count, int64(11))

	str, err := c.Get(ctx, "mykey")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, str, "Hello World")
}
