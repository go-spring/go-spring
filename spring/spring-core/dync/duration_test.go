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

func TestDuration(t *testing.T) {

	var d dync.Duration
	assert.Equal(t, d.Value(), time.Duration(0))

	param := conf.BindParam{
		Key:  "d",
		Path: "Duration",
		Tag: conf.ParsedTag{
			Key: "d",
		},
	}

	p := conf.Map(nil)
	err := d.OnRefresh(p, param)
	assert.Error(t, err, "bind Duration error; .* resolve property \"d\" error; property \"d\" not exist")

	_ = p.Set("d", "10s")

	param.Validate = ""
	err = d.OnRefresh(p, param)
	assert.Equal(t, d.Value(), 10*time.Second)

	b, err := json.Marshal(&d)
	assert.Nil(t, err)
	assert.Equal(t, string(b), "10000000000")
}
