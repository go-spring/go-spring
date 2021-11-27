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

package cast_test

import (
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func TestToTime(t *testing.T) {

	s := cast.ToTime(1, cast.TimeArg{Unit: time.Nanosecond})
	assert.Equal(t, s, time.Unix(0, 1))

	s = cast.ToTime(1, cast.TimeArg{Unit: time.Millisecond})
	assert.Equal(t, s, time.Unix(0, 1*1e6))

	s = cast.ToTime(1, cast.TimeArg{Unit: time.Second})
	assert.Equal(t, s, time.Unix(1, 0))

	s = cast.ToTime(1, cast.TimeArg{Unit: time.Hour})
	assert.Equal(t, s, time.Unix(0, 0).Add(time.Hour))

	format := "2006-01-02 15:04:05.000000000 -0700"
	ts := "1970-01-01 08:00:00.000000001 +0800"
	s = cast.ToTime(ts, cast.TimeArg{Format: format})
	assert.Equal(t, s, time.Unix(0, 1))
}
