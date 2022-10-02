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
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/clock"
)

func TestToStdLayout(t *testing.T) {
	layouts := []struct {
		Custom string
		Native string
	}{
		{
			Custom: "yyyy",
			Native: "2006",
		},
		{
			Custom: "MM",
			Native: "01",
		},
		{
			Custom: "dd",
			Native: "02",
		},
		{
			Custom: "D",
			Native: "002",
		},
		{
			Custom: "H",
			Native: "15",
		},
		{
			Custom: "h",
			Native: "03",
		},
		{
			Custom: "m",
			Native: "04",
		},
		{
			Custom: "s",
			Native: "05",
		},
		{
			Custom: "yyyy-MM-dd H:m:s",
			Native: "2006-01-02 15:04:05",
		},
	}
	for _, layout := range layouts {
		format := clock.ToStdLayout(layout.Custom)
		assert.Equal(t, layout.Native, format)
	}
}

func TestFormat(t *testing.T) {
	layouts := []struct {
		Custom string
		Native string
	}{
		{
			Custom: "",
			Native: "",
		},
		{
			Custom: "yyyy",
			Native: "2006",
		},
		{
			Custom: "MM",
			Native: "01",
		},
		{
			Custom: "MMM",
			Native: "Sep",
		},
		{
			Custom: "dd",
			Native: "02",
		},
		{
			Custom: "D",
			Native: "002",
		},
		{Custom: "H", Native: "15"},
		{
			Custom: "h",
			Native: "03",
		},
		{Custom: "m", Native: "04"},
		{
			Custom: "s",
			Native: "05",
		},
		{
			Custom: "yyyy-MM-dd H:m:s",
			Native: "2006-01-02 15:04:05",
		},
	}
	now := time.Date(2022, 9, 30, 15, 30, 0, 0, time.UTC)
	for _, layout := range layouts {
		native := now.Format(layout.Native)
		custom := clock.Format(now, layout.Custom)
		assert.Equal(t, native, custom)
	}
}

func TestUnitFormat(t *testing.T) {
	layouts := []struct {
		Custom string
		Native string
	}{
		{
			Custom: "yyyy年",
			Native: "2006年",
		},
		{
			Custom: "yy年",
			Native: "06年",
		},
		{
			Custom: "MM月",
			Native: "01月",
		},
		{
			Custom: "dd日",
			Native: "02日",
		},
		{
			Custom: "D年天",
			Native: "002年天",
		},
		{
			Custom: "H24小时",
			Native: "1524小时",
		},
		{
			Custom: "h12小时",
			Native: "0312小时",
		},
		{
			Custom: "m分钟",
			Native: "04分钟",
		},
		{
			Custom: "s秒数",
			Native: "05秒数",
		},
		{
			Custom: "yyyy年MM月dd日 H时m分s秒",
			Native: "2006年01月02日 15时04分05秒",
		},
	}
	now := time.Date(2022, 9, 30, 15, 30, 0, 0, time.UTC)
	for _, layout := range layouts {
		native := now.Format(layout.Native)
		custom := clock.Format(now, layout.Custom)
		assert.Equal(t, native, custom)
	}
}

func BenchmarkFormat(b *testing.B) {
	// BenchmarkFormat/native-8 4811834 263.5 ns/op
	// BenchmarkFormat/custom-8 2654034 442.3 ns/op
	b.Run("native", func(b *testing.B) {
		now := time.Now()
		for i := 0; i < b.N; i++ {
			now.Format("2006-01-02 15:04:05")
		}
	})
	b.Run("custom", func(b *testing.B) {
		now := time.Now()
		for i := 0; i < b.N; i++ {
			clock.Format(now, "yyyy-MM-dd H:m:s")
		}
	})
}
