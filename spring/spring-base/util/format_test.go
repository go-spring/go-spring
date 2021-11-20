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
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
)

func Test_Format(t *testing.T) {
	layouts := []struct {
		Custom string
		Native string
	}{
		{"d", "02"},
		{"D", "Mon"},
		{"j", "1"},
		{"Y", "2006"},
		{"y", "06"},
		{"m", "01"},
		{"M", "Jan"},
		{"a", "pm"},
		{"A", "PM"},
		{"H", "15"},
		{"h", "3"},
		{"i", "04"},
		{"s", "05"},
		{"Y-m-d H:i:s", "2006-01-02 15:04:05"},
	}

	now := time.Now()

	for _, layout := range layouts {
		custom := util.Format(now, layout.Custom)
		native := now.Format(layout.Native)
		assert.Equal(t, native, custom)
	}
}

// BenchmarkFormat
// BenchmarkFormat/native
// BenchmarkFormat/native-8         	 4865997	       217 ns/op
// BenchmarkFormat/custom
// BenchmarkFormat/custom-8         	 2150994	       551 ns/op
func BenchmarkFormat(b *testing.B) {
	now := time.Now()

	b.Run("native", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			now.Format("2006-01-02 15:04:05")
		}
	})

	b.Run("custom", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			util.Format(now, "Y-m-d H:i:s")
		}
	})
}
