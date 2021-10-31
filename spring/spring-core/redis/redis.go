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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/util"
)

var ErrNil = errors.New("redis: nil")

type Client interface {
	BaseCommand
	KeyCommand
	BitmapCommand
	StringCommand
	HashCommand
	ListCommand
	SetCommand
	ZSetCommand
	ServerCommand
}

type BaseCommand interface {
	Bool(ctx context.Context, args ...interface{}) (bool, error)
	Int64(ctx context.Context, args ...interface{}) (int64, error)
	Float64(ctx context.Context, args ...interface{}) (float64, error)
	String(ctx context.Context, args ...interface{}) (string, error)
	Slice(ctx context.Context, args ...interface{}) ([]interface{}, error)
	BoolSlice(ctx context.Context, args ...interface{}) ([]bool, error)
	Int64Slice(ctx context.Context, args ...interface{}) ([]int64, error)
	Float64Slice(ctx context.Context, args ...interface{}) ([]float64, error)
	StringSlice(ctx context.Context, args ...interface{}) ([]string, error)
	ZItemSlice(ctx context.Context, args ...interface{}) ([]ZItem, error)
	StringMap(ctx context.Context, args ...interface{}) (map[string]string, error)
}

type Reply interface {
	Value() interface{}
	Bool() (bool, error)
	Int64() (int64, error)
	Float64() (float64, error)
	String() (string, error)
	Slice() ([]interface{}, error)
	BoolSlice() ([]bool, error)
	Int64Slice() ([]int64, error)
	Float64Slice() ([]float64, error)
	StringSlice() ([]string, error)
	ZItemSlice() ([]ZItem, error)
	StringMap() (map[string]string, error)
}

type BaseClient struct {
	DoFunc func(ctx context.Context, args ...interface{}) (Reply, error)
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

func (c *BaseClient) do(ctx context.Context, args ...interface{}) (r Reply, err error) {

	defer func() {
		if err == nil && fastdev.RecordMode() {
			fastdev.RecordAction(ctx, &fastdev.Action{
				Protocol: fastdev.REDIS,
				Request:  cmdString(args),
				Response: cast.ToString(r.Value()),
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
		return &reply{action.Response}, nil
	}

	return c.DoFunc(ctx, args...)
}

func (c *BaseClient) Bool(ctx context.Context, args ...interface{}) (bool, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return false, err
	}
	return reply.Bool()
}

func (c *BaseClient) Int64(ctx context.Context, args ...interface{}) (int64, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return -1, err
	}
	return reply.Int64()
}

func (c *BaseClient) Float64(ctx context.Context, args ...interface{}) (float64, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return -1, err
	}
	return reply.Float64()
}

func (c *BaseClient) String(ctx context.Context, args ...interface{}) (string, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return "", err
	}
	return reply.String()
}

func (c *BaseClient) Slice(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.Slice()
}

func (c *BaseClient) BoolSlice(ctx context.Context, args ...interface{}) ([]bool, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.BoolSlice()
}

func (c *BaseClient) Int64Slice(ctx context.Context, args ...interface{}) ([]int64, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.Int64Slice()
}

func (c *BaseClient) Float64Slice(ctx context.Context, args ...interface{}) ([]float64, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.Float64Slice()
}

func (c *BaseClient) StringSlice(ctx context.Context, args ...interface{}) ([]string, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.StringSlice()
}

func (c *BaseClient) ZItemSlice(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.ZItemSlice()
}

func (c *BaseClient) StringMap(ctx context.Context, args ...interface{}) (map[string]string, error) {
	reply, err := c.do(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.StringMap()
}

type reply struct {
	v interface{}
}

func (r *reply) Value() interface{} {
	panic(util.UnsupportedMethod)
}

func (r *reply) Bool() (bool, error) {
	switch v := r.v.(type) {
	case int64:
		return v == 1, nil
	case string:
		return v == "OK", nil
	default:
		return false, fmt.Errorf("redis: unexpected type %T for bool", v)
	}
}

func (r *reply) Int64() (int64, error) {
	return cast.ToInt64E(r.v)
}

func (r *reply) Float64() (float64, error) {
	return cast.ToFloat64E(r.v)
}

func (r *reply) String() (string, error) {
	return cast.ToStringE(r.v)
}

func (r *reply) toValue(v interface{}) error {
	return json.Unmarshal([]byte(cast.ToString(r.v)), v)
}

func (r *reply) Slice() (v []interface{}, err error) {
	if err = r.toValue(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func (r *reply) BoolSlice() (v []bool, err error) {
	if err = r.toValue(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func (r *reply) Int64Slice() (v []int64, err error) {
	if err = r.toValue(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func (r *reply) Float64Slice() (v []float64, err error) {
	if err = r.toValue(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func (r *reply) StringSlice() (v []string, err error) {
	if err = r.toValue(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func (r *reply) ZItemSlice() (v []ZItem, err error) {
	if err = r.toValue(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func (r *reply) StringMap() (v map[string]string, err error) {
	if err = r.toValue(&v); err != nil {
		return nil, err
	}
	return v, nil
}
