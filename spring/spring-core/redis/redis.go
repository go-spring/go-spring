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

type Reply interface {
	Bool() bool
	Int64() int64
	Float64() float64
	String() string
	Slice() []interface{}
	Int64Slice() []int64
	Float64Slice() []float64
	BoolSlice() []bool
	StringSlice() []string
	StringMap() map[string]string
}

type BaseCommand interface {
	Bool(ctx context.Context, args ...interface{}) (bool, error)
	Int64(ctx context.Context, args ...interface{}) (int64, error)
	Float64(ctx context.Context, args ...interface{}) (float64, error)
	String(ctx context.Context, args ...interface{}) (string, error)
	Slice(ctx context.Context, args ...interface{}) ([]interface{}, error)
	Int64Slice(ctx context.Context, args ...interface{}) ([]int64, error)
	Float64Slice(ctx context.Context, args ...interface{}) ([]float64, error)
	BoolSlice(ctx context.Context, args ...interface{}) ([]bool, error)
	StringSlice(ctx context.Context, args ...interface{}) ([]string, error)
	StringMap(ctx context.Context, args ...interface{}) (map[string]string, error)
}

type BaseClient struct {
	DoFunc func(ctx context.Context, args ...interface{}) (Reply, error)
}

func (c *BaseClient) Bool(ctx context.Context, args ...interface{}) (bool, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return false, err
	}
	return reply.Bool(), nil
}

func (c *BaseClient) Int64(ctx context.Context, args ...interface{}) (int64, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return -1, err
	}
	return reply.Int64(), nil
}

func (c *BaseClient) Float64(ctx context.Context, args ...interface{}) (float64, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return -1, err
	}
	return reply.Float64(), nil
}

func (c *BaseClient) String(ctx context.Context, args ...interface{}) (string, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return "", err
	}
	return reply.String(), nil
}

func (c *BaseClient) Slice(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.Slice(), nil
}

func (c *BaseClient) Int64Slice(ctx context.Context, args ...interface{}) ([]int64, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.Int64Slice(), nil
}

func (c *BaseClient) Float64Slice(ctx context.Context, args ...interface{}) ([]float64, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.Float64Slice(), nil
}

func (c *BaseClient) BoolSlice(ctx context.Context, args ...interface{}) ([]bool, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.BoolSlice(), nil
}

func (c *BaseClient) StringSlice(ctx context.Context, args ...interface{}) ([]string, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.StringSlice(), nil
}

func (c *BaseClient) StringMap(ctx context.Context, args ...interface{}) (map[string]string, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.StringMap(), nil
}
