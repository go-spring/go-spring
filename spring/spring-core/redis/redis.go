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

//go:generate mockgen -build_flags="-mod=mod" -package=redis -source=redis.go -destination=redis_mock.go

// Package redis provides operations for redis commands.
package redis

import (
	"context"
	"fmt"
	"strconv"
)

// IsOK returns whether s equals "OK".
func IsOK(s string) bool {
	return "OK" == s
}

// Config Is the configuration of redis client.
type Config struct {
	Host           string `value:"${host:=127.0.0.1}"`
	Port           int    `value:"${port:=6379}"`
	Username       string `value:"${username:=}"`
	Password       string `value:"${password:=}"`
	Database       int    `value:"${database:=0}"`
	Ping           bool   `value:"${ping:=true}"`
	IdleTimeout    int    `value:"${idle-timeout:=0}"`
	ConnectTimeout int    `value:"${connect-timeout:=0}"`
	ReadTimeout    int    `value:"${read-timeout:=0}"`
	WriteTimeout   int    `value:"${write-timeout:=0}"`
}

type Result struct {
	Data []string
}

// NewResult returns a new *Result.
func NewResult(data ...string) *Result {
	return &Result{Data: data}
}

type Driver interface {
	Exec(ctx context.Context, args []interface{}) (interface{}, error)
}

var (
	Recorder func(Driver) Driver
	Replayer func(Driver) Driver
)

// Client provides operations for redis commands.
type Client struct {
	driver Driver
}

// NewClient returns a new *Client.
func NewClient(driver Driver) *Client {
	if Recorder != nil {
		driver = Recorder(driver)
	}
	if Replayer != nil {
		driver = Replayer(driver)
	}
	return &Client{driver: driver}
}

func toInt64(v interface{}, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	switch r := v.(type) {
	case nil:
		return 0, nil
	case int64:
		return r, nil
	case float64:
		return int64(r), nil
	case string:
		return strconv.ParseInt(r, 10, 64)
	case *Result:
		if len(r.Data) == 0 {
			return 0, fmt.Errorf("redis: no data")
		}
		return toInt64(r.Data[0], nil)
	default:
		return 0, fmt.Errorf("redis: unexpected type (%T) for int64", v)
	}
}

// Int executes a command whose reply is a `int64`.
func (c *Client) Int(ctx context.Context, args ...interface{}) (int64, error) {
	return toInt64(c.driver.Exec(ctx, args))
}

func toFloat64(v interface{}, err error) (float64, error) {
	if err != nil {
		return 0, err
	}
	switch r := v.(type) {
	case nil:
		return 0, nil
	case float64:
		return r, nil
	case int64:
		return float64(r), nil
	case string:
		return strconv.ParseFloat(r, 64)
	case *Result:
		if len(r.Data) == 0 {
			return 0, fmt.Errorf("redis: no data")
		}
		return toFloat64(r.Data[0], nil)
	default:
		return 0, fmt.Errorf("redis: unexpected type (%T) for float64", r)
	}
}

// Float executes a command whose reply is a `float64`.
func (c *Client) Float(ctx context.Context, args ...interface{}) (float64, error) {
	return toFloat64(c.driver.Exec(ctx, args))
}

func toString(v interface{}, err error) (string, error) {
	if err != nil {
		return "", err
	}
	switch r := v.(type) {
	case nil:
		return "", nil
	case string:
		return r, nil
	case *Result:
		if len(r.Data) == 0 {
			return "", fmt.Errorf("redis: no data")
		}
		return r.Data[0], nil
	default:
		return "", fmt.Errorf("redis: unexpected type (%T) for string", v)
	}
}

// String executes a command whose reply is a `string`.
func (c *Client) String(ctx context.Context, args ...interface{}) (string, error) {
	return toString(c.driver.Exec(ctx, args))
}

func toSlice(v interface{}, err error) ([]interface{}, error) {
	if err != nil {
		return nil, err
	}
	switch r := v.(type) {
	case nil:
		return nil, nil
	case []interface{}:
		return r, nil
	case []string:
		if len(r) == 0 {
			return nil, nil
		}
		slice := make([]interface{}, len(r))
		for i, str := range r {
			if str == "NULL" {
				slice[i] = nil
			} else {
				slice[i] = str
			}
		}
		return slice, nil
	case *Result:
		return toSlice(r.Data, nil)
	default:
		return nil, fmt.Errorf("redis: unexpected type (%T) for []interface{}", v)
	}
}

// Slice executes a command whose reply is a `[]interface{}`.
func (c *Client) Slice(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	return toSlice(c.driver.Exec(ctx, args))
}

func toInt64Slice(v interface{}, err error) ([]int64, error) {
	slice, err := toSlice(v, err)
	if err != nil {
		return nil, err
	}
	if len(slice) == 0 {
		return nil, nil
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

// IntSlice executes a command whose reply is a `[]int64`.
func (c *Client) IntSlice(ctx context.Context, args ...interface{}) ([]int64, error) {
	return toInt64Slice(c.driver.Exec(ctx, args))
}

func toFloat64Slice(v interface{}, err error) ([]float64, error) {
	slice, err := toSlice(v, err)
	if err != nil {
		return nil, err
	}
	if len(slice) == 0 {
		return nil, nil
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

// FloatSlice executes a command whose reply is a `[]float64`.
func (c *Client) FloatSlice(ctx context.Context, args ...interface{}) ([]float64, error) {
	return toFloat64Slice(c.driver.Exec(ctx, args))
}

func toStringSlice(v interface{}, err error) ([]string, error) {
	slice, err := toSlice(v, err)
	if err != nil {
		return nil, err
	}
	if len(slice) == 0 {
		return nil, nil
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

// StringSlice executes a command whose reply is a `[]string`.
func (c *Client) StringSlice(ctx context.Context, args ...interface{}) ([]string, error) {
	return toStringSlice(c.driver.Exec(ctx, args))
}

func toStringMap(v interface{}, err error) (map[string]string, error) {
	if err != nil {
		return nil, err
	}
	slice, err := toStringSlice(v, err)
	if err != nil {
		return nil, err
	}
	if len(slice) == 0 {
		return nil, nil
	}
	if len(slice)%2 != 0 {
		return nil, fmt.Errorf("redis: unexpected slice length %d", len(slice))
	}
	val := make(map[string]string, len(slice)/2)
	for i := 0; i < len(slice); i += 2 {
		val[slice[i]] = slice[i+1]
	}
	return val, nil
}

// StringMap executes a command whose reply is a `map[string]string`.
func (c *Client) StringMap(ctx context.Context, args ...interface{}) (map[string]string, error) {
	return toStringMap(c.driver.Exec(ctx, args))
}

func toZItemSlice(v interface{}, err error) ([]ZItem, error) {
	if err != nil {
		return nil, err
	}
	slice, err := toStringSlice(v, err)
	if err != nil {
		return nil, err
	}
	if len(slice) == 0 {
		return nil, nil
	}
	if len(slice)%2 != 0 {
		return nil, fmt.Errorf("redis: unexpected slice length %d", len(slice))
	}
	val := make([]ZItem, len(slice)/2)
	for i := 0; i < len(val); i++ {
		idx := i * 2
		var score float64
		score, err = toFloat64(slice[idx+1], nil)
		if err != nil {
			return nil, err
		}
		val[i].Member = slice[idx]
		val[i].Score = score
	}
	return val, nil
}

// ZItemSlice executes a command whose reply is a `[]ZItem`.
func (c *Client) ZItemSlice(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	return toZItemSlice(c.driver.Exec(ctx, args))
}
