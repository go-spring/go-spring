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

package log_test

import (
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/log"
)

func TestDenyAllFilter(t *testing.T) {
	f := log.DenyAllFilter{}
	assert.Equal(t, f.Filter(log.TraceLevel, nil, nil), log.ResultDeny)
}

func TestLevelFilter(t *testing.T) {
	t.Run("", func(t *testing.T) {
		f := log.LevelFilter{
			BaseFilter: log.BaseFilter{
				OnMatch:    log.ResultAccept,
				OnMismatch: log.ResultDeny,
			},
			Level: log.InfoLevel,
		}
		assert.Equal(t, f.Filter(log.TraceLevel, nil, nil), log.ResultDeny)
		assert.Equal(t, f.Filter(log.InfoLevel, nil, nil), log.ResultAccept)
		assert.Equal(t, f.Filter(log.ErrorLevel, nil, nil), log.ResultAccept)
	})
	t.Run("", func(t *testing.T) {
		f := log.LevelFilter{
			BaseFilter: log.BaseFilter{
				OnMatch:    log.ResultDeny,
				OnMismatch: log.ResultAccept,
			},
			Level: log.InfoLevel,
		}
		assert.Equal(t, f.Filter(log.TraceLevel, nil, nil), log.ResultAccept)
		assert.Equal(t, f.Filter(log.InfoLevel, nil, nil), log.ResultDeny)
		assert.Equal(t, f.Filter(log.ErrorLevel, nil, nil), log.ResultDeny)
	})
}

func TestLevelMatchFilter(t *testing.T) {
	t.Run("", func(t *testing.T) {
		f := log.LevelMatchFilter{
			BaseFilter: log.BaseFilter{
				OnMatch:    log.ResultAccept,
				OnMismatch: log.ResultDeny,
			},
			Level: log.InfoLevel,
		}
		assert.Equal(t, f.Filter(log.TraceLevel, nil, nil), log.ResultDeny)
		assert.Equal(t, f.Filter(log.InfoLevel, nil, nil), log.ResultAccept)
		assert.Equal(t, f.Filter(log.ErrorLevel, nil, nil), log.ResultDeny)
	})
	t.Run("", func(t *testing.T) {
		f := log.LevelMatchFilter{
			BaseFilter: log.BaseFilter{
				OnMatch:    log.ResultDeny,
				OnMismatch: log.ResultAccept,
			},
			Level: log.InfoLevel,
		}
		assert.Equal(t, f.Filter(log.TraceLevel, nil, nil), log.ResultAccept)
		assert.Equal(t, f.Filter(log.InfoLevel, nil, nil), log.ResultDeny)
		assert.Equal(t, f.Filter(log.ErrorLevel, nil, nil), log.ResultAccept)
	})
}

func TestLevelRangeFilter(t *testing.T) {
	t.Run("", func(t *testing.T) {
		f := log.LevelRangeFilter{
			BaseFilter: log.BaseFilter{
				OnMatch:    log.ResultAccept,
				OnMismatch: log.ResultDeny,
			},
			MinLevel: log.InfoLevel,
			MaxLevel: log.ErrorLevel,
		}
		assert.Equal(t, f.Filter(log.TraceLevel, nil, nil), log.ResultDeny)
		assert.Equal(t, f.Filter(log.InfoLevel, nil, nil), log.ResultAccept)
		assert.Equal(t, f.Filter(log.ErrorLevel, nil, nil), log.ResultAccept)
		assert.Equal(t, f.Filter(log.PanicLevel, nil, nil), log.ResultDeny)
	})
	t.Run("", func(t *testing.T) {
		f := log.LevelRangeFilter{
			BaseFilter: log.BaseFilter{
				OnMatch:    log.ResultDeny,
				OnMismatch: log.ResultAccept,
			},
			MinLevel: log.InfoLevel,
			MaxLevel: log.ErrorLevel,
		}
		assert.Equal(t, f.Filter(log.TraceLevel, nil, nil), log.ResultAccept)
		assert.Equal(t, f.Filter(log.InfoLevel, nil, nil), log.ResultDeny)
		assert.Equal(t, f.Filter(log.ErrorLevel, nil, nil), log.ResultDeny)
		assert.Equal(t, f.Filter(log.PanicLevel, nil, nil), log.ResultAccept)
	})
}

func TestTimeFilter(t *testing.T) {
	t.Run("", func(t *testing.T) {
		f := &log.TimeFilter{
			BaseFilter: log.BaseFilter{
				OnMatch:    log.ResultAccept,
				OnMismatch: log.ResultDeny,
			},
			Timezone: "Local",
			Start:    "11:00:00",
			End:      "18:00:00",
		}
		if err := f.Init(); err != nil {
			t.Fatal(err)
		}
		testcases := []struct {
			time   []int
			expect log.Result
		}{
			{
				time:   []int{10, 59, 59},
				expect: log.ResultDeny,
			},
			{
				time:   []int{11, 00, 00},
				expect: log.ResultAccept,
			},
			{
				time:   []int{18, 00, 00},
				expect: log.ResultAccept,
			},
			{
				time:   []int{18, 00, 01},
				expect: log.ResultDeny,
			},
		}
		for _, c := range testcases {
			f.TimeFunc = func() time.Time {
				date, month, day := time.Now().Date()
				return time.Date(date, month, day, c.time[0], c.time[1], c.time[2], 0, time.Local)
			}
			assert.Equal(t, f.Filter(log.TraceLevel, nil, nil), c.expect)
		}
	})
	t.Run("", func(t *testing.T) {
		f := &log.TimeFilter{
			BaseFilter: log.BaseFilter{
				OnMatch:    log.ResultDeny,
				OnMismatch: log.ResultAccept,
			},
			Timezone: "Local",
			Start:    "11:00:00",
			End:      "18:00:00",
		}
		if err := f.Init(); err != nil {
			t.Fatal(err)
		}
		testcases := []struct {
			time   []int
			expect log.Result
		}{
			{
				time:   []int{10, 59, 59},
				expect: log.ResultAccept,
			},
			{
				time:   []int{11, 00, 00},
				expect: log.ResultDeny,
			},
			{
				time:   []int{18, 00, 00},
				expect: log.ResultDeny,
			},
			{
				time:   []int{18, 00, 01},
				expect: log.ResultAccept,
			},
		}
		for _, c := range testcases {
			f.TimeFunc = func() time.Time {
				date, month, day := time.Now().Date()
				return time.Date(date, month, day, c.time[0], c.time[1], c.time[2], 0, time.Local)
			}
			assert.Equal(t, f.Filter(log.TraceLevel, nil, nil), c.expect)
		}
	})
}
