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
)

func TestNow(t *testing.T) {

	ctx, _ := knife.New(context.Background())
	trueNow := time.Now()

	t.Run("SetFixedTime", func(t *testing.T) {
		err := clock.SetFixedTime(ctx, time.Unix(100, 0))
		assert.Nil(t, err)
		defer clock.ResetTime(ctx)
		now := clock.Now(ctx)
		assert.Equal(t, now, time.Unix(100, 0))
	})

	t.Run("SetBaseTime", func(t *testing.T) {
		err := clock.SetBaseTime(ctx, time.Unix(100, 0))
		assert.Nil(t, err)
		defer clock.ResetTime(ctx)
		time.Sleep(300 * time.Millisecond)
		now := clock.Now(ctx)
		assert.True(t, now.Sub(time.Unix(100, 0)) < 500*time.Millisecond)
	})

	assert.True(t, clock.Now(ctx).Sub(trueNow) < 500*time.Millisecond)
}
