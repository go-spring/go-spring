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

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/recorder"
	"github.com/go-spring/spring-base/replayer"
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

func (c *BaseClient) do(ctx context.Context, args ...interface{}) (Reply, error) {

	key := func() string {
		var buf bytes.Buffer
		for i, arg := range args {
			buf.WriteString(cast.ToString(arg))
			if i < len(args)-1 {
				buf.WriteByte('^')
			}
		}
		return buf.String()
	}

	fmt.Println(key())

	if replayer.ReplayMode() {
		data := &recorder.Redis{Request: args}
		action := &recorder.Action{
			Protocol: recorder.REDIS,
			Key:      key(),
			Data:     data,
		}
		ok, err := replayer.Replay(ctx, action)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("xxx")
		}
		return &reply{data.Response}, nil
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
	v, ok := r.v.(int64)
	if !ok {
		return 0, fmt.Errorf("redis: unexpected type %T for int64", r.v)
	}
	return v, nil
}

func (r *reply) Float64() (float64, error) {
	v, ok := r.v.(float64)
	if !ok {
		return 0, fmt.Errorf("redis: unexpected type %T for float64", r.v)
	}
	return v, nil
}

func (r *reply) String() (string, error) {
	v, ok := r.v.(string)
	if !ok {
		return "", fmt.Errorf("redis: unexpected type %T for string", r.v)
	}
	return v, nil
}

func (r *reply) Slice() ([]interface{}, error) {
	v, ok := r.v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("redis: unexpected type %T for []interface{}", r.v)
	}
	return v, nil
}

func (r *reply) BoolSlice() ([]bool, error) {
	v, ok := r.v.([]bool)
	if !ok {
		return nil, fmt.Errorf("redis: unexpected type %T for []bool", r.v)
	}
	return v, nil
}

func (r *reply) Int64Slice() ([]int64, error) {
	v, ok := r.v.([]int64)
	if !ok {
		return nil, fmt.Errorf("redis: unexpected type %T for []int64", r.v)
	}
	return v, nil
}

func (r *reply) Float64Slice() ([]float64, error) {
	v, ok := r.v.([]float64)
	if !ok {
		return nil, fmt.Errorf("redis: unexpected type %T for []float64", r.v)
	}
	return v, nil
}

func (r *reply) StringSlice() ([]string, error) {
	v, ok := r.v.([]string)
	if !ok {
		return nil, fmt.Errorf("redis: unexpected type %T for []string", r.v)
	}
	return v, nil
}

func (r *reply) ZItemSlice() ([]ZItem, error) {
	v, ok := r.v.([]ZItem)
	if !ok {
		return nil, fmt.Errorf("redis: unexpected type %T for []redis.ZItem", r.v)
	}
	return v, nil
}

func (r *reply) StringMap() (map[string]string, error) {
	v, ok := r.v.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("redis: unexpected type %T for map[string]string", r.v)
	}
	return v, nil
}
