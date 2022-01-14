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

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/chrono"
	"github.com/go-spring/spring-base/fastdev"
)

type recordConn struct {
	conn Conn
}

func (c *recordConn) Exec(ctx context.Context, args ...interface{}) (ret interface{}, err error) {

	defer func() {
		if fastdev.RecordMode() {
			var resp interface{}
			if err == nil {
				resp = ret
			} else if IsErrNil(err) {
				resp = "(nil)"
			} else {
				resp = "(err) " + err.Error()
			}
			fastdev.RecordAction(ctx, &fastdev.Action{
				Protocol:  fastdev.REDIS,
				Request:   cast.CmdString(args),
				Response:  resp,
				Timestamp: chrono.Now(ctx).UnixNano(),
			})
		}
	}()

	return c.conn.Exec(ctx, args...)
}

type replayConn struct{}

func (c *replayConn) Exec(ctx context.Context, args ...interface{}) (interface{}, error) {

	action := &fastdev.Action{
		Protocol: fastdev.REDIS,
		Request:  cast.CmdString(args),
	}

	ok, err := fastdev.ReplayAction(ctx, action, func(r1, r2 interface{}) bool {
		return r1 == r2
	})
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("replay action not match")
	}

	if action.Response == "(nil)" {
		return nil, ErrNil()
	}

	s, ok := action.Response.(string)
	if ok {
		if strings.HasPrefix(s, "(err) ") {
			return nil, errors.New(strings.TrimPrefix(s, "(err) "))
		}
	}
	return action.Response, nil
}
