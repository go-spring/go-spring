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

package atomic_test

import (
	"testing"
	"time"
	"unsafe"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/json"
)

func TestTime(t *testing.T) {

	// atomic.Time and interface{} occupy the same space
	assert.Equal(t, unsafe.Sizeof(atomic.Time{}), uintptr(16+8))

	var tm atomic.Time
	assert.Equal(t, tm.Load(), time.Time{})

	tm.Store(time.Unix(1, 1).UTC())
	assert.Equal(t, tm.Load(), time.Unix(1, 1).UTC())

	bytes, _ := json.Marshal(&tm)
	assert.Equal(t, string(bytes), `"Thu Jan  1 00:00:01 UTC 1970"`)

	tm.SetMarshalJSON(func(t time.Time) ([]byte, error) {
		return json.Marshal(t.Format("20060102"))
	})

	bytes, _ = json.Marshal(&tm)
	assert.Equal(t, string(bytes), `"19700101"`)
}
