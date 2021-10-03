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

package util

import (
	"context"
	"errors"
	"time"
)

// MilliSeconds 返回 time.Time 的毫秒时间。
func MilliSeconds(t time.Time) int64 {
	return t.UnixNano() / 1e6
}

type TimeKey int

type TimeValue struct {
	base time.Time
	from time.Time
}

var timeKey TimeKey

// Now 返回当前时间。
func Now(ctx context.Context) time.Time {
	if ctx == nil {
		return time.Now()
	}
	t, ok := ctx.Value(timeKey).(*TimeValue)
	if !ok {
		return time.Now()
	}
	return t.base.Add(time.Now().Sub(t.from))
}

// MockNow 模拟当前时间。
func MockNow(ctx context.Context, t time.Time) context.Context {
	if _, ok := ctx.Value(timeKey).(*TimeValue); ok {
		panic(errors.New("time value already mocked"))
	}
	return context.WithValue(ctx, timeKey, &TimeValue{
		base: t,
		from: time.Now(),
	})
}
