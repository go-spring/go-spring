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

package clock_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/run"
)

func TestNow(t *testing.T) {

	t.Run("normal_mode", func(t *testing.T) {
		reset := run.SetMode(run.NormalModeFlag)
		defer func() { reset() }()
		ctx, _ := knife.New(context.Background())
		knife.Store(ctx, "::now::", time.Now().Add(time.Hour))
		assert.True(t, clock.Now(ctx).Sub(time.Now()) < time.Millisecond)
	})

	t.Run("record_mode", func(t *testing.T) {
		reset := run.SetMode(run.RecordModeFlag)
		defer func() { reset() }()
		ctx, _ := knife.New(context.Background())
		knife.Store(ctx, "::now::", time.Now().Add(time.Hour))
		assert.True(t, clock.Now(ctx).Sub(time.Now()) < time.Millisecond)
	})

	t.Run("null_time", func(t *testing.T) {
		ctx, _ := knife.New(context.Background())
		knife.Store(ctx, "::now::", time.Now().Add(time.Hour))
		assert.True(t, clock.Now(ctx).Sub(time.Now()) < time.Millisecond)
	})

	t.Run("base_time", func(t *testing.T) {
		ctx, _ := knife.New(context.Background())
		trueNow := time.Now()

		err := clock.SetBaseTime(ctx, time.Unix(100, 0))
		assert.Nil(t, err)

		time.Sleep(5 * time.Millisecond)
		n := clock.Now(ctx).Sub(time.Unix(100, 0))
		assert.True(t, n > 5*time.Millisecond && n < 10*time.Millisecond)

		clock.ResetTime(ctx)
		assert.True(t, clock.Now(ctx).Sub(trueNow) < 20*time.Millisecond)
	})

	t.Run("fixed_time", func(t *testing.T) {
		ctx, _ := knife.New(context.Background())
		trueNow := time.Now()

		err := clock.SetFixedTime(ctx, time.Unix(100, 0))
		assert.Nil(t, err)

		now := clock.Now(ctx)
		assert.Equal(t, now, time.Unix(100, 0))

		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, now, time.Unix(100, 0))

		clock.ResetTime(ctx)
		assert.True(t, clock.Now(ctx).Sub(trueNow) < 20*time.Millisecond)
	})
}
