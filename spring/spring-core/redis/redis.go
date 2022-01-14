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

// 在当前目录下执行下面的命令可以生成 mock 代码，
// mockgen -build_flags="-mod=mod" -package=redis -destination=mock.go github.com/go-spring/spring-core/redis Client

package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-core/internal"
)

var errNil = errors.New("redis: nil")

func OK(s string) bool {
	return "OK" == s
}

func ErrNil() error {
	return errNil
}

func IsErrNil(err error) bool {
	return errors.Is(err, errNil)
}

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

type ClientConfig = internal.RedisClientConfig

type Driver interface {
	Open(config ClientConfig) (Conn, error)
}

type Conn interface {
	Exec(ctx context.Context, args ...interface{}) (interface{}, error)
}

type client struct {
	conn Conn
}

func NewClient(config ClientConfig, d Driver) (Client, error) {

	var (
		conn Conn
		err  error
	)

	if fastdev.ReplayMode() {
		conn = &replayConn{}
	} else {
		conn, err = d.Open(config)
		if err != nil {
			return nil, err
		}
	}

	if fastdev.RecordMode() {
		conn = &recordConn{conn: conn}
	}
	return &client{conn: conn}, nil
}

type transform func(interface{}, error) (interface{}, error)

func (c *client) do(ctx context.Context, args []interface{}, trans transform) (ret interface{}, err error) {
	if trans == nil {
		return c.conn.Exec(ctx, args...)
	}
	return trans(c.conn.Exec(ctx, args...))
}

func toInt(v interface{}, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	switch r := v.(type) {
	case int64:
		return int(r), nil
	case float64:
		return int(r), nil
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for int64", v)
	}
}

func (c *client) Int(ctx context.Context, args ...interface{}) (int, error) {
	return toInt(c.do(ctx, args, nil))
}

func toInt64(v interface{}, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	switch r := v.(type) {
	case int64:
		return r, nil
	case float64:
		return int64(r), nil
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for int64", v)
	}
}

func (c *client) Int64(ctx context.Context, args ...interface{}) (int64, error) {
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

func (c *client) Float64(ctx context.Context, args ...interface{}) (float64, error) {
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

func (c *client) String(ctx context.Context, args ...interface{}) (string, error) {
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

func (c *client) Slice(ctx context.Context, args ...interface{}) ([]interface{}, error) {
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

func (c *client) Int64Slice(ctx context.Context, args ...interface{}) ([]int64, error) {
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

func (c *client) Float64Slice(ctx context.Context, args ...interface{}) ([]float64, error) {
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

func (c *client) StringSlice(ctx context.Context, args ...interface{}) ([]string, error) {
	return toStringSlice(c.do(ctx, args, nil))
}

func toStringMap(v interface{}, err error) (map[string]string, error) {
	if err != nil {
		return nil, err
	}
	switch r := v.(type) {
	case map[string]string:
		return r, nil
	case map[string]interface{}:
		ret := make(map[string]string)
		for key, val := range r {
			var str string
			str, err = toString(val, nil)
			if err != nil {
				return nil, err
			}
			ret[key] = str
		}
		return ret, nil
	default:
		return nil, fmt.Errorf("redis: unexpected type %T for map[string]string", v)
	}
}

func (c *client) StringMap(ctx context.Context, args ...interface{}) (map[string]string, error) {
	return toStringMap(c.do(ctx, args, func(v interface{}, err error) (interface{}, error) {
		slice, err := toStringSlice(v, err)
		if err != nil {
			return nil, err
		}
		val := make(map[string]string, len(slice)/2)
		for i := 0; i < len(slice); i += 2 {
			val[slice[i]] = slice[i+1]
		}
		return val, nil
	}))
}

func toZItemSlice(v interface{}, err error) ([]ZItem, error) {
	if err != nil {
		return nil, err
	}
	switch r := v.(type) {
	case [][]string:
		val := make([]ZItem, len(r))
		for i := 0; i < len(val); i++ {
			var score float64
			score, err = toFloat64(r[i][1], nil)
			if err != nil {
				return nil, err
			}
			val[i].Member = r[i][0]
			val[i].Score = score
		}
		return val, nil
	case []interface{}:
		val := make([]ZItem, len(r))
		for i := 0; i < len(val); i++ {
			var slice []interface{}
			slice, err = toSlice(r[i], nil)
			if err != nil {
				return nil, err
			}
			if len(slice) != 2 {
				return nil, errors.New("redis: error replay data")
			}
			var score float64
			score, err = toFloat64(slice[1], nil)
			if err != nil {
				return nil, err
			}
			val[i].Member = slice[0]
			val[i].Score = score
		}
		return val, nil
	default:
		return nil, fmt.Errorf("redis: unexpected type %T for []ZItem", v)
	}
}

func (c *client) ZItemSlice(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	return toZItemSlice(c.do(ctx, args, func(v interface{}, err error) (interface{}, error) {
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
	}))
}
