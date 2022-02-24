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

package chrono

import (
	"context"
	"time"

	"github.com/go-spring/spring-base/knife"
)

const nowKey = "::now::"

type FakeTime interface {
	Now() time.Time
}

type fixedTime struct {
	fixed time.Time
}

func (t *fixedTime) Now() time.Time {
	return t.fixed
}

type baseTime struct {
	base time.Time
	from time.Time
}

func (t *baseTime) Now() time.Time {
	return t.base.Add(time.Since(t.from))
}

// ResetTime 恢复正常时间。
func ResetTime(ctx context.Context) {
	knife.Delete(ctx, nowKey)
}

// SetFixedTime 设置固定时间。
func SetFixedTime(ctx context.Context, t time.Time) error {
	return knife.Store(ctx, nowKey, &fixedTime{fixed: t})
}

// SetBaseTime 设置基准时间。
func SetBaseTime(ctx context.Context, t time.Time) error {
	return knife.Store(ctx, nowKey, &baseTime{base: t, from: time.Now()})
}

// Now 获取当前时间。
func Now(ctx context.Context) time.Time {
	if ctx == nil {
		return time.Now()
	}
	v, err := knife.Load(ctx, nowKey)
	if err != nil || v == nil {
		return time.Now()
	}
	t, ok := v.(FakeTime)
	if !ok {
		return time.Now()
	}
	return t.Now()
}

// MilliSeconds 返回 time.Time 的毫秒时间。
func MilliSeconds(t time.Time) int64 {
	return t.UnixNano() / 1e6
}
