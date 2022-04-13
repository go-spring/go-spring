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

package replayer_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/net/recorder"
	"github.com/go-spring/spring-base/net/replayer"
)

func init() {
	recorder.RegisterProtocol(recorder.HTTP, &httpProtocol{})
	recorder.RegisterProtocol(recorder.REDIS, &redisProtocol{})
}

func TestReplayAction(t *testing.T) {

	replayer.SetReplayMode(true)
	defer func() {
		replayer.SetReplayMode(false)
	}()

	agent := replayer.NewLocalAgent()
	replayer.SetReplayAgent(agent)

	sessionID := "39fc5c13443f47da9ff320cc4b02c789"
	ctx, _ := knife.New(context.Background())
	err := replayer.SetSessionID(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	recordSession := &recorder.Session{
		Session:   sessionID,
		Timestamp: clock.Now(ctx).UnixNano(),
		Inbound: &recorder.Action{
			Protocol:  recorder.HTTP,
			Timestamp: clock.Now(ctx).UnixNano(),
			Request: recorder.Message(func() string {
				return "GET ..."
			}),
			Response: recorder.Message(func() string {
				return "200 ..."
			}),
		},
		Actions: []*recorder.Action{
			{
				Protocol:  recorder.REDIS,
				Timestamp: clock.Now(ctx).UnixNano(),
				Request: recorder.Message(func() string {
					return recorder.EncodeTTY("SET", "a", "1")
				}),
				Response: recorder.Message(func() string {
					return recorder.EncodeCSV(1, "2", 3)
				}),
			}, {
				Protocol:  recorder.REDIS,
				Timestamp: clock.Now(ctx).UnixNano(),
				Request: recorder.Message(func() string {
					return recorder.EncodeTTY("SET", "a", "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n")
				}),
				Response: recorder.Message(func() string {
					return recorder.EncodeCSV("\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n")
				}),
			},
			{
				Protocol:  recorder.REDIS,
				Timestamp: clock.Now(ctx).UnixNano(),
				Request: recorder.Message(func() string {
					return recorder.EncodeTTY("HGET", "a")
				}),
				Response: recorder.Message(func() string {
					return recorder.EncodeCSV("a", "b", "c", 3, "d", "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n")
				}),
			},
		},
	}

	str := recorder.ToPrettyJson(recordSession)
	fmt.Println("record:", str)

	session, err := agent.Store(str)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Print("json(record): ")
	fmt.Println(session.Pretty())

	defer agent.Delete(session.Session)

	request := recorder.EncodeTTY("SET", "a", "1")
	response, _, _ := replayer.Query(ctx, recorder.REDIS, request)
	assert.Equal(t, response, "\"1\",\"2\",\"3\"")

	request = recorder.EncodeTTY("SET", "a", "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n")
	response, _, _ = replayer.Query(ctx, recorder.REDIS, request)
	assert.Equal(t, response, "\"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\"")

	err = session.Flat()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(session.Pretty())

	assert.Equal(t, session.Actions[0].FlatRequest, map[string]string{
		"$[0]": "SET",
		"$[1]": "a",
		"$[2]": "1",
	})
	assert.Equal(t, session.Actions[0].FlatResponse, map[string]string{
		"$[0]": "1",
		"$[1]": "2",
		"$[2]": "3",
	})
	assert.Equal(t, session.Actions[1].FlatRequest, map[string]string{
		"$[0]": "SET",
		"$[1]": "a",
		"$[2]": "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n",
	})
	assert.Equal(t, session.Actions[1].FlatResponse, map[string]string{
		"$[0]": "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n",
	})
}

type httpProtocol struct{}

func (p *httpProtocol) ShouldDiff() bool {
	return true
}

func (p *httpProtocol) GetLabel(data string) string {
	return data[:4]
}

func (p *httpProtocol) FlatRequest(data string) (map[string]string, error) {
	return nil, nil
}

func (p *httpProtocol) FlatResponse(data string) (map[string]string, error) {
	return nil, nil
}

type redisProtocol struct{}

func (p *redisProtocol) ShouldDiff() bool {
	return true
}

func (p *redisProtocol) GetLabel(data string) string {
	return data[:4]
}

func (p *redisProtocol) FlatRequest(data string) (map[string]string, error) {
	csv, err := recorder.DecodeTTY(data)
	if err != nil {
		return nil, err
	}
	return recorder.FlatJSON(csv), nil
}

func (p *redisProtocol) FlatResponse(data string) (map[string]string, error) {
	csv, err := recorder.DecodeCSV(data)
	if err != nil {
		return nil, err
	}
	return recorder.FlatJSON(csv), nil
}
