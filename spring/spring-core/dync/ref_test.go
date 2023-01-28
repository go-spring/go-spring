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

func TestRef_Uint32(t *testing.T) {

	var r dync.Ref
	assert.Equal(t, r.Value(), nil)

	r.Init(uint32(0))

	var count int
	r.OnEvent = func() error {
		count++
		return nil
	}

	param := conf.BindParam{
		Key:  "uint",
		Path: "uint32",
		Tag: conf.ParsedTag{
			Key: "uint",
		},
	}

	p := conf.Map(nil)
	err := r.OnRefresh(p, param)
	assert.Error(t, err, "bind uint32 error; .* resolve property \"uint\" error; property \"uint\" not exist")
	assert.Equal(t, count, 0)

	_ = p.Set("uint", uint32(3))

	param.Validate = "$>5"
	err = r.OnRefresh(p, param)
	assert.Error(t, err, "validate failed on \"\\$\\>5\" for value 3")
	assert.Equal(t, count, 0)

	param.Validate = ""
	err = r.OnRefresh(p, param)
	assert.Equal(t, r.Value(), uint32(3))
	assert.Equal(t, count, 1)

	r.OnEvent = nil
	param.Validate = ""
	err = r.OnRefresh(p, param)
	assert.Equal(t, r.Value(), uint32(3))
	assert.Equal(t, count, 1)

	b, err := json.Marshal(&r)
	assert.Nil(t, err)
	assert.Equal(t, string(b), "3")
}

func TestRef_Duration(t *testing.T) {

	var r dync.Ref
	assert.Equal(t, r.Value(), nil)

	r.Init(time.Duration(0))

	var count int
	r.OnEvent = func() error {
		count++
		return nil
	}

	param := conf.BindParam{
		Key:  "d",
		Path: "Duration",
		Tag: conf.ParsedTag{
			Key: "d",
		},
	}

	p := conf.Map(nil)
	err := r.OnRefresh(p, param)
	assert.Error(t, err, "bind Duration error; .* resolve property \"d\" error; property \"d\" not exist")
	assert.Equal(t, count, 0)

	_ = p.Set("d", "10s")

	param.Validate = ""
	err = r.OnRefresh(p, param)
	assert.Equal(t, r.Value(), 10*time.Second)
	assert.Equal(t, count, 1)

	b, err := json.Marshal(&r)
	assert.Nil(t, err)
	assert.Equal(t, string(b), "10000000000")
}

func TestRef_Time(t *testing.T) {

	var r dync.Ref
	assert.Equal(t, r.Value(), nil)

	r.Init(time.Time{})

	var count int
	r.OnEvent = func() error {
		count++
		return nil
	}

	param := conf.BindParam{
		Key:  "time",
		Path: "Time",
		Tag: conf.ParsedTag{
			Key: "time",
		},
	}

	p := conf.Map(nil)
	err := r.OnRefresh(p, param)
	assert.Error(t, err, "bind Time error; .* resolve property \"time\" error; property \"time\" not exist")
	assert.Equal(t, count, 0)

	_ = p.Set("time", "2017-06-17 13:20:15 UTC")

	param.Validate = "" // TODO validate
	err = r.OnRefresh(p, param)
	assert.Nil(t, err)
	assert.Equal(t, count, 1)

	r.OnEvent = nil
	param.Validate = ""
	err = r.OnRefresh(p, param)
	assert.Equal(t, r.Value(), time.Date(2017, 6, 17, 13, 20, 15, 0, time.UTC))
	assert.Equal(t, count, 1)

	b, err := json.Marshal(&r)
	assert.Nil(t, err)
	assert.Equal(t, string(b), "\"2017-06-17T13:20:15Z\"")
}
