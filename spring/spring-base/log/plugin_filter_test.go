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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/log"
)

type mockFilter struct {
	start  func() error
	result log.Result
}

func (f *mockFilter) Start() error {
	if f.start != nil {
		return f.start()
	}
	return nil
}

func (f *mockFilter) Stop(ctx context.Context) {

}

func (f *mockFilter) Filter(e *log.Event) log.Result {
	return f.result
}

func TestCompositeFilter(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		filter := log.CompositeFilter{
			Filters: []log.Filter{
				&mockFilter{start: func() error {
					return errors.New("start failed")
				}},
			},
		}
		err := filter.Start()
		assert.Error(t, err, "start failed")
	})
	//t.Run("success", func(t *testing.T) {
	//	filter := log.CompositeFilter{
	//		Filters: []log.Filter{
	//			&log.LevelRangeFilter{
	//				Min: log.DebugLevel,
	//				Max: log.InfoLevel,
	//			},
	//			&log.LevelMatchFilter{
	//				Level: log.PanicLevel,
	//			},
	//		},
	//	}
	//	err := filter.Start()
	//	assert.Nil(t, err)
	//	filter.Stop(context.Background())
	//	v := filter.Filter(&log.Event{Level: log.TraceLevel})
	//	assert.Equal(t, v, log.ResultDeny)
	//	v = filter.Filter(&log.Event{Level: log.DebugLevel})
	//	assert.Equal(t, v, log.ResultAccept)
	//	v = filter.Filter(&log.Event{Level: log.InfoLevel})
	//	assert.Equal(t, v, log.ResultAccept)
	//	v = filter.Filter(&log.Event{Level: log.WarnLevel})
	//	assert.Equal(t, v, log.ResultDeny)
	//	v = filter.Filter(&log.Event{Level: log.ErrorLevel})
	//	assert.Equal(t, v, log.ResultDeny)
	//	v = filter.Filter(&log.Event{Level: log.PanicLevel})
	//	assert.Equal(t, v, log.ResultAccept)
	//	v = filter.Filter(&log.Event{Level: log.FatalLevel})
	//	assert.Equal(t, v, log.ResultDeny)
	//})
}

func TestDenyAllFilter(t *testing.T) {
	f := log.DenyAllFilter{}
	assert.Equal(t, f.Filter(nil), log.ResultDeny)
}

func TestLevelFilter(t *testing.T) {
	f := log.LevelFilter{Level: log.InfoLevel}
	assert.Equal(t, f.Filter(&log.Event{Level: log.TraceLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.DebugLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.InfoLevel}), log.ResultAccept)
	assert.Equal(t, f.Filter(&log.Event{Level: log.WarnLevel}), log.ResultAccept)
	assert.Equal(t, f.Filter(&log.Event{Level: log.ErrorLevel}), log.ResultAccept)
	assert.Equal(t, f.Filter(&log.Event{Level: log.PanicLevel}), log.ResultAccept)
	assert.Equal(t, f.Filter(&log.Event{Level: log.FatalLevel}), log.ResultAccept)
}

func TestLevelMatchFilter(t *testing.T) {
	f := log.LevelMatchFilter{Level: log.InfoLevel}
	assert.Equal(t, f.Filter(&log.Event{Level: log.TraceLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.DebugLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.InfoLevel}), log.ResultAccept)
	assert.Equal(t, f.Filter(&log.Event{Level: log.WarnLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.ErrorLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.PanicLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.FatalLevel}), log.ResultDeny)
}

func TestLevelRangeFilter(t *testing.T) {
	f := log.LevelRangeFilter{Min: log.InfoLevel, Max: log.ErrorLevel}
	assert.Equal(t, f.Filter(&log.Event{Level: log.TraceLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.DebugLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.InfoLevel}), log.ResultAccept)
	assert.Equal(t, f.Filter(&log.Event{Level: log.WarnLevel}), log.ResultAccept)
	assert.Equal(t, f.Filter(&log.Event{Level: log.ErrorLevel}), log.ResultAccept)
	assert.Equal(t, f.Filter(&log.Event{Level: log.PanicLevel}), log.ResultDeny)
	assert.Equal(t, f.Filter(&log.Event{Level: log.FatalLevel}), log.ResultDeny)
}

func TestTimeFilter(t *testing.T) {
	f := &log.TimeFilter{
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
		ctx, _ := knife.New(context.Background())
		year, month, day := time.Now().Date()
		date := time.Date(year, month, day, c.time[0], c.time[1], c.time[2], 0, time.Local)
		_ = clock.SetFixedTime(ctx, date)
		//entry := new(log.Entry).WithContext(ctx)
		//assert.Equal(t, f.Filter(nil), c.expect)
	}
}
