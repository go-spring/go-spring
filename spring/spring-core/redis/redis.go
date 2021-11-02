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
	"bytes"
	"context"
	"errors"
	"strconv"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev"
)

var ErrNil = errors.New("redis: nil")

type Client interface {
	DoCommand
	KeyCommand
	BitmapCommand
	StringCommand
	HashCommand
	ListCommand
	SetCommand
	ZSetCommand
	ServerCommand
}

type DoCommand interface {
	Do(ctx context.Context, args ...interface{}) (interface{}, error)
}

type BaseClient struct {
	DoFunc func(ctx context.Context, args ...interface{}) (interface{}, error)
}

// needQuote 判断是否需要双引号包裹。
func needQuote(s string) bool {
	for _, c := range s {
		switch c {
		case '"', '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0:
			return true
		}
	}
	return false
}

func cmdString(args []interface{}) string {
	var buf bytes.Buffer
	for i, arg := range args {
		switch s := arg.(type) {
		case string:
			if needQuote(s) {
				s = strconv.Quote(s)
			}
			buf.WriteString(s)
		default:
			buf.WriteString(cast.ToString(arg))
		}
		if i < len(args)-1 {
			buf.WriteByte(' ')
		}
	}
	return buf.String()
}

func (c *BaseClient) Do(ctx context.Context, args ...interface{}) (r interface{}, err error) {

	defer func() {
		if err == nil && fastdev.RecordMode() {
			fastdev.RecordAction(ctx, &fastdev.Action{
				Protocol: fastdev.REDIS,
				Request:  cmdString(args),
				Response: cast.ToString(r),
			})
		}
	}()

	if fastdev.ReplayMode() {
		action := &fastdev.Action{
			Protocol: fastdev.REDIS,
			Request:  cmdString(args),
		}
		var ok bool
		ok, err = fastdev.ReplayAction(ctx, action)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("replay action not match")
		}
		return action.Response, nil
	}

	return c.DoFunc(ctx, args...)
}
