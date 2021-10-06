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
)

type Reply interface {
	Bool() bool
	Int64() int64
	String() string
	StringSlice() []string
}

type Client interface {
	KeyCommand
	BitmapCommand
	StringCommand
	HashCommand
	ListCommand
	SetCommand
	Do(ctx context.Context, args ...interface{}) (Reply, error)
}

type BaseClient struct {
	DoFunc func(ctx context.Context, args ...interface{}) (Reply, error)
}

func (c *BaseClient) Do(ctx context.Context, args ...interface{}) (Reply, error) {
	return c.DoFunc(ctx, args...)
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

func (c *BaseClient) String(ctx context.Context, args ...interface{}) (string, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return "", err
	}
	return reply.String(), nil
}

func (c *BaseClient) StringSlice(ctx context.Context, args ...interface{}) ([]string, error) {
	reply, err := c.DoFunc(ctx, args...)
	if err != nil {
		return nil, err
	}
	return reply.StringSlice(), nil
}
