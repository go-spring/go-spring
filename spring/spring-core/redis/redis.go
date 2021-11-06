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
	"fmt"
	"strconv"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev"
)

const OK = "OK"

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
	Int(ctx context.Context, args ...interface{}) (int, error)
	Int64(ctx context.Context, args ...interface{}) (int64, error)
	Float64(ctx context.Context, args ...interface{}) (float64, error)
	String(ctx context.Context, args ...interface{}) (string, error)
	Slice(ctx context.Context, args ...interface{}) ([]interface{}, error)
	Int64Slice(ctx context.Context, args ...interface{}) ([]int64, error)
	Float64Slice(ctx context.Context, args ...interface{}) ([]float64, error)
	StringSlice(ctx context.Context, args ...interface{}) ([]string, error)
	StringMap(ctx context.Context, args ...interface{}) (map[string]string, error)
	ZItemSlice(ctx context.Context, args ...interface{}) ([]ZItem, error)
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
			} else {
				q := strconv.Quote(s)
				if q[1:len(q)-1] != s {
					s = q
				}
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

type transform func(interface{}, error) (interface{}, error)

func (c *BaseClient) do(ctx context.Context, args []interface{}, trans transform) (r interface{}, err error) {

	defer func() {
		if fastdev.RecordMode() {
			var resp interface{}
			if err == nil {
				resp = r
			} else if err == ErrNil {
				resp = "(nil)"
			} else {
				resp = "(err) " + err.Error()
			}
			fastdev.RecordAction(ctx, &fastdev.Action{
				Protocol: fastdev.REDIS,
				Request:  cmdString(args),
				Response: resp,
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

	if trans == nil {
		return c.DoFunc(ctx, args...)
	}
	return trans(c.DoFunc(ctx, args...))
}

func toInt(v interface{}, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	switch r := v.(type) {
	case int64:
		return int(r), nil
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for int64", v)
	}
}

func (c *BaseClient) Int(ctx context.Context, args ...interface{}) (int, error) {
	return toInt(c.do(ctx, args, nil))
}

func toInt64(v interface{}, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	switch r := v.(type) {
	case int64:
		return r, nil
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for int64", v)
	}
}

func (c *BaseClient) Int64(ctx context.Context, args ...interface{}) (int64, error) {
	return toInt64(c.do(ctx, args, nil))
}

func toFloat64(v interface{}, err error) (float64, error) {
	if err != nil {
		return 0, err
	}
	switch r := v.(type) {
	case nil:
		return 0, nil
	case int64:
		return float64(r), nil
	case string:
		return strconv.ParseFloat(r, 64)
	default:
		return 0, fmt.Errorf("redis: unexpected type=%T for float64", r)
	}
}

func (c *BaseClient) Float64(ctx context.Context, args ...interface{}) (float64, error) {
	return toFloat64(c.do(ctx, args, nil))
}

func toString(v interface{}, err error) (string, error) {
	if err != nil {
		return "", err
	}
	switch r := v.(type) {
	case string:
		return r, nil
	default:
		return "", fmt.Errorf("redis: unexpected type %T for string", v)
	}
}

func (c *BaseClient) String(ctx context.Context, args ...interface{}) (string, error) {
	return toString(c.do(ctx, args, nil))
}

func toSlice(v interface{}, err error) ([]interface{}, error) {
	if err != nil {
		return nil, err
	}
	switch r := v.(type) {
	case []interface{}:
		return r, nil
	default:
		return nil, fmt.Errorf("redis: unexpected type %T for []interface{}", v)
	}
}

func (c *BaseClient) Slice(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	return toSlice(c.do(ctx, args, nil))
}

func toInt64Slice(v interface{}, err error) ([]int64, error) {
	slice, err := toSlice(v, err)
	if err != nil {
		return nil, err
	}
	val := make([]int64, len(slice))
	for i, r := range slice {
		var n int64
		n, err = toInt64(r, nil)
		if err != nil {
			return nil, err
		}
		val[i] = n
	}
	return val, nil
}

func (c *BaseClient) Int64Slice(ctx context.Context, args ...interface{}) ([]int64, error) {
	return toInt64Slice(c.do(ctx, args, nil))
}

func toFloat64Slice(v interface{}, err error) ([]float64, error) {
	slice, err := toSlice(v, err)
	if err != nil {
		return nil, err
	}
	val := make([]float64, len(slice))
	for i, r := range slice {
		var f float64
		f, err = toFloat64(r, nil)
		if err != nil {
			return nil, err
		}
		val[i] = f
	}
	return val, nil
}

func (c *BaseClient) Float64Slice(ctx context.Context, args ...interface{}) ([]float64, error) {
	return toFloat64Slice(c.do(ctx, args, nil))
}

func toStringSlice(v interface{}, err error) ([]string, error) {
	slice, err := toSlice(v, err)
	if err != nil {
		return nil, err
	}
	val := make([]string, len(slice))
	for i, r := range slice {
		var str string
		str, err = toString(r, nil)
		if err != nil {
			return nil, err
		}
		val[i] = str
	}
	return val, nil
}

func (c *BaseClient) StringSlice(ctx context.Context, args ...interface{}) ([]string, error) {
	return toStringSlice(c.do(ctx, args, nil))
}

func (c *BaseClient) StringMap(ctx context.Context, args ...interface{}) (map[string]string, error) {
	v, err := c.do(ctx, args, func(v interface{}, err error) (interface{}, error) {
		slice, err := toStringSlice(v, err)
		if err != nil {
			return nil, err
		}
		val := make(map[string]string, len(slice)/2)
		for i := 0; i < len(slice); i += 2 {
			val[slice[i]] = slice[i+1]
		}
		return val, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(map[string]string), nil
}

func (c *BaseClient) ZItemSlice(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	v, err := c.do(ctx, args, func(v interface{}, err error) (interface{}, error) {
		slice, err := toStringSlice(v, err)
		if err != nil {
			return nil, err
		}
		val := make([][]string, len(slice)/2)
		for i := 0; i < len(val); i++ {
			idx := i * 2
			val[i] = []string{slice[idx], slice[idx+1]}
		}
		return val, nil
	})
	if err != nil {
		return nil, err
	}
	slice := v.([][]string)
	val := make([]ZItem, len(slice))
	for i := 0; i < len(val); i++ {
		member := slice[i][0]
		var score float64
		score, err = strconv.ParseFloat(slice[i][1], 64)
		if err != nil {
			return nil, err
		}
		val[i].Member = member
		val[i].Score = score
	}
	return val, nil
}
