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

package dync_test

import (
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/json"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/dync"
)

func TestTime(t *testing.T) {

	var tm dync.Time
	assert.Equal(t, tm.Value(), time.Time{})

	param := conf.BindParam{
		Key:  "time",
		Path: "Time",
		Tag: conf.ParsedTag{
			Key: "time",
		},
	}

	p := conf.Map(nil)
	err := tm.OnRefresh(p, param)
	assert.Error(t, err, "bind Time error; .* resolve property \"time\" error; property \"time\" not exist")

	_ = p.Set("time", "2017-06-17 13:20:15 UTC")

	param.Validate = "" // TODO validate
	err = tm.OnRefresh(p, param)
	assert.Nil(t, err)

	param.Validate = ""
	err = tm.OnRefresh(p, param)
	assert.Equal(t, tm.Value(), time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC))

	b, err := json.Marshal(&tm)
	assert.Nil(t, err)
	assert.Equal(t, string(b), "\"2017-06-17T13:20:15Z\"")
}
