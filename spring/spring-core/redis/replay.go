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

package redis

import (
	"context"
	"errors"
	"strings"

	"github.com/go-spring/spring-base/net/recorder"
	"github.com/go-spring/spring-base/net/replayer"
)

func init() {
	recorder.RegisterProtocol(recorder.REDIS, &protocol{})
}

type replayResult struct {
	data []string
}

type replayConn struct {
	conn Conn
}

func (c *replayConn) Exec(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {

	req := recorder.EncodeTTY(append([]interface{}{cmd}, args...)...)
	response, ok, err := replayer.BestQuery(ctx, recorder.REDIS, req)
	if err != nil {
		return nil, err
	}
	if !ok {
		return c.conn.Exec(ctx, cmd, args)
	}

	csv, err := recorder.DecodeCSV(response)
	if err != nil {
		return nil, err
	}

	if len(csv) == 1 {
		s := csv[0]
		if s == "NULL" {
			return nil, ErrNil()
		}
		if strings.HasPrefix(s, "(err) ") {
			return nil, errors.New(strings.TrimPrefix(s, "(err) "))
		}
	}

	return &replayResult{csv}, nil
}

type protocol struct{}

func (p *protocol) GetLabel(data string) string {
	return strings.SplitN(data, " ", 2)[0]
}

func (p *protocol) FlatRequest(data string) (map[string]string, error) {
	csv, err := recorder.DecodeTTY(data)
	if err != nil {
		return nil, err
	}
	return recorder.FlatJSON(csv), nil
}

func (p *protocol) FlatResponse(data string) (map[string]string, error) {
	csv, err := recorder.DecodeCSV(data)
	if err != nil {
		return nil, err
	}
	return recorder.FlatJSON(csv), nil
}
