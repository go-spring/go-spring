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

package guava_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/chrono"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/recorder"
	"github.com/go-spring/spring-base/fastdev/replayer"
	"github.com/go-spring/spring-base/guava"
	"github.com/go-spring/spring-base/knife"
)

func init() {
	fastdev.RegisterProtocol(fastdev.REDIS, &redisProtocol{})
}

type response struct {
	Name string `json:"name"`
}

type redis struct {
	count int
}

func (r *redis) getValue(ctx context.Context, key string) (ret string, err error) {
	r.count++
	defer func() {
		if recorder.EnableRecord(ctx) {
			recorder.RecordAction(ctx, &fastdev.Action{
				Protocol: fastdev.REDIS,
				Request: fastdev.NewMessage(func() string {
					return key
				}),
				Response: fastdev.NewMessage(func() string {
					return ret
				}),
			})
		}
	}()
	if replayer.ReplayMode() {
		var resp interface{}
		resp, err = replayer.QueryAction(ctx, fastdev.REDIS, key, replayer.BestMatch)
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", errors.New("no replay data")
		}
		return resp.(string), nil
	}
	return fmt.Sprintf("{\"name\":\"%s\"}", key), nil
}

func getResponse(ctx context.Context, r *redis, key string) (*response, error) {
	loadType, result, err := guava.Load(ctx, key, 0, func(ctx context.Context, key string) (interface{}, error) {
		data, err := r.getValue(ctx, key)
		if err != nil {
			return nil, err
		}
		var v *response
		err = json.Unmarshal([]byte(data), &v)
		if err != nil {
			return nil, err
		}
		return v, nil
	})
	if err != nil {
		return nil, err
	}
	var resp *response
	err = result.Load(&resp)
	if err != nil {
		return nil, err
	}
	fmt.Println(loadType)
	return resp, nil
}

func testFunc(t *testing.T, ctx context.Context, key string, count int) {

	if recorder.RecordMode() {
		sessionID := "39fc5c13443f47da9ff320cc4b02c789"
		var err error
		ctx, err = recorder.StartRecord(ctx, sessionID)
		assert.Nil(t, err)
	}

	if replayer.ReplayMode() {
		sessionID := "39fc5c13443f47da9ff320cc4b02c789"
		err := replayer.SetSessionID(ctx, sessionID)
		assert.Nil(t, err)
	}

	r := &redis{}
	for i := 0; i < 3; i++ {
		resp, err := getResponse(ctx, r, key)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("%#v\n", resp)
	}
	assert.Equal(t, r.count, count)

	if recorder.EnableRecord(ctx) {
		session, err := recorder.StopRecord(ctx)
		assert.Nil(t, err)
		fmt.Println(session.PrettyJson())
	}
}

func TestGuavaRecord(t *testing.T) {

	recorder.SetRecordMode(true)
	defer func() {
		recorder.SetRecordMode(false)
	}()

	key := "test"
	f := func(str string, count int) {

		ctx, cached := knife.New(context.Background())
		assert.False(t, cached)

		timeNow := time.Unix(1643364150, 0)
		err := chrono.SetBaseTime(ctx, timeNow)
		assert.Nil(t, err)

		testFunc(t, ctx, key, count)
	}

	f(``, 1)
	f(``, 0)
	f(``, 0)
}

func TestGuavaReplay(t *testing.T) {

	replayer.SetReplayMode(true)
	defer func() {
		replayer.SetReplayMode(false)
	}()

	key := "test"
	f := func(str string, count int) {

		agent := replayer.NewLocalAgent()
		replayer.SetReplayAgent(agent)

		session, err := agent.Store(str)
		assert.Nil(t, err)
		defer agent.Delete(session.Session)

		ctx, cached := knife.New(context.Background())
		assert.False(t, cached)

		timeNow := time.Unix(1643364150, 0)
		err = chrono.SetBaseTime(ctx, timeNow)
		assert.Nil(t, err)

		testFunc(t, ctx, key, count)
	}

	f(`
	{
	  "Session": "39fc5c13443f47da9ff320cc4b02c789",
	  "Timestamp": 1643364150000015602,
	  "Actions": [
		{
		  "Protocol": "REDIS",
		  "Timestamp": 1643364150000015602,
		  "Request": "test",
		  "Response": "{\"name\":\"test\"}"
		}
	  ]
	}`, 1)

	f(`
	{
	  "Session": "39fc5c13443f47da9ff320cc4b02c789",
	  "Timestamp": 1643364150000001919,
	  "Actions": [
		{
		  "Protocol": "APCU",
		  "Timestamp": 1643364150000001919,
		  "Request": "test",
		  "Response": "{\"name\":\"test\"}"
		}
	  ]
	}`, 0)

	f(`
	{
	  "Session": "39fc5c13443f47da9ff320cc4b02c789",
	  "Timestamp": 1643364150000001503,
	  "Actions": [
		{
		  "Protocol": "APCU",
		  "Timestamp": 1643364150000001503,
		  "Request": "test",
		  "Response": "{\"name\":\"test\"}"
		}
	  ]
	}`, 0)
}

type redisProtocol struct{}

func (p *redisProtocol) ShouldDiff() bool {
	return true
}

func (p *redisProtocol) GetLabel(data string) string {
	return data[:4]
}

func (p *redisProtocol) FlatRequest(data string) (map[string]string, error) {
	return nil, nil
}

func (p *redisProtocol) FlatResponse(data string) (map[string]string, error) {
	return nil, nil
}
