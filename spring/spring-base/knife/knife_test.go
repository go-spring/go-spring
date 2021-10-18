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

package knife_test

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/knife"
)

func TestGet(t *testing.T) {
	ctx := context.Background()

	v, ok := knife.Get(ctx, "a")
	assert.False(t, ok)

	err := knife.Set(ctx, "a", "b")
	assert.Equal(t, err, knife.ErrUninitialized)

	v, ok = knife.Get(ctx, "a")
	assert.False(t, ok)

	ctx = knife.New(ctx)

	v, ok = knife.Get(ctx, "a")
	assert.False(t, ok)

	err = knife.Set(ctx, "a", "b")
	assert.Nil(t, err)

	v, ok = knife.Get(ctx, "a")
	assert.Equal(t, v, "b")
}

func TestFetch(t *testing.T) {
	ctx := knife.New(context.Background())

	err := knife.Set(ctx, "a", map[string]string{"b": "c"})
	assert.Nil(t, err)

	var m map[string]string
	ok, err := knife.Fetch(ctx, "a", &m)
	assert.True(t, ok)

	var b bool
	ok, err = knife.Fetch(ctx, "a", &b)
	assert.False(t, ok)
	assert.Error(t, err, "want bool but got map\\[string]string")
}
