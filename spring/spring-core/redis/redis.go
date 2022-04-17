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
	"fmt"
	"strconv"

	"github.com/go-spring/spring-base/net/recorder"
	"github.com/go-spring/spring-base/net/replayer"
)

var errNil = errors.New("redis: nil")

func IsOK(s string) bool {
	return "OK" == s
}

func ErrNil() error {
	return errNil
}

func IsErrNil(err error) bool {
	return errors.Is(err, errNil)
}

type Redis interface {
	Int(ctx context.Context, cmd string, args ...interface{}) (int, error)
	Int64(ctx context.Context, cmd string, args ...interface{}) (int64, error)
	Float64(ctx context.Context, cmd string, args ...interface{}) (float64, error)
	String(ctx context.Context, cmd string, args ...interface{}) (string, error)
	Slice(ctx context.Context, cmd string, args ...interface{}) ([]interface{}, error)
	Int64Slice(ctx context.Context, cmd string, args ...interface{}) ([]int64, error)
	Float64Slice(ctx context.Context, cmd string, args ...interface{}) ([]float64, error)
	StringSlice(ctx context.Context, cmd string, args ...interface{}) ([]string, error)
	StringMap(ctx context.Context, cmd string, args ...interface{}) (map[string]string, error)
	ZItemSlice(ctx context.Context, cmd string, args ...interface{}) ([]ZItem, error)
}

type Client struct {
	conn Conn
}

func NewClient(config Config, driver Driver) (*Client, error) {
	conn, err := driver.Open(config)
	if err != nil {
		return nil, err
	}
	if recorder.RecordMode() {
		conn = &recordConn{conn: conn}
	}
	if replayer.ReplayMode() {
		conn = &replayConn{conn: conn}
	}
	return &Client{conn: conn}, nil
}

func (c *Client) KeyCommand() *KeyCommand {
	return NewKeyCommand(c)
}

func (c *Client) BitmapCommand() *BitmapCommand {
	return NewBitmapCommand(c)
}

func (c *Client) StringCommand() *StringCommand {
	return NewStringCommand(c)
}

func (c *Client) HashCommand() *HashCommand {
	return NewHashCommand(c)
}

func (c *Client) ListCommand() *ListCommand {
	return NewListCommand(c)
}

func (c *Client) SetCommand() *SetCommand {
	return NewSetCommand(c)
}

func (c *Client) ZSetCommand() *ZSetCommand {
	return NewZSetCommand(c)
}

func (c *Client) ServerCommand() *ServerCommand {
	return NewServerCommand(c)
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
	case string:
		var n int64
		n, err = strconv.ParseInt(r, 10, 64)
		if err != nil {
			return 0, err
		}
		return int(n), nil
	case *replayResult:
		if len(r.data) == 0 {
			return 0, fmt.Errorf("redis: no data")
		}
		return toInt(r.data[0], nil)
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for int64", v)
	}
}

func (c *Client) Int(ctx context.Context, cmd string, args ...interface{}) (int, error) {
	return toInt(c.conn.Exec(ctx, cmd, args))
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
	case string:
		return strconv.ParseInt(r, 10, 64)
	case *replayResult:
		if len(r.data) == 0 {
			return 0, fmt.Errorf("redis: no data")
		}
		return toInt64(r.data[0], nil)
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for int64", v)
	}
}

func (c *Client) Int64(ctx context.Context, cmd string, args ...interface{}) (int64, error) {
	return toInt64(c.conn.Exec(ctx, cmd, args))
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
	case *replayResult:
		if len(r.data) == 0 {
			return 0, fmt.Errorf("redis: no data")
		}
		return toFloat64(r.data[0], nil)
	default:
		return 0, fmt.Errorf("redis: unexpected type=%T for float64", r)
	}
}

func (c *Client) Float64(ctx context.Context, cmd string, args ...interface{}) (float64, error) {
	return toFloat64(c.conn.Exec(ctx, cmd, args))
}

func toString(v interface{}, err error) (string, error) {
	if err != nil {
		return "", err
	}
	switch r := v.(type) {
	case string:
		return r, nil
	case *replayResult:
		if len(r.data) == 0 {
			return "", fmt.Errorf("redis: no data")
		}
		return r.data[0], nil
	default:
		return "", fmt.Errorf("redis: unexpected type %T for string", v)
	}
}

func (c *Client) String(ctx context.Context, cmd string, args ...interface{}) (string, error) {
	return toString(c.conn.Exec(ctx, cmd, args))
}

func toSlice(v interface{}, err error) ([]interface{}, error) {
	if err != nil {
		return nil, err
	}
	switch r := v.(type) {
	case []interface{}:
		return r, nil
	case []string:
		var slice []interface{}
		for _, str := range r {
			if str == "NULL" {
				slice = append(slice, nil)
			} else {
				slice = append(slice, str)
			}
		}
		return slice, nil
	case *replayResult:
		return toSlice(r.data, nil)
	default:
		return nil, fmt.Errorf("redis: unexpected type %T for []interface{}", v)
	}
}

func (c *Client) Slice(ctx context.Context, cmd string, args ...interface{}) ([]interface{}, error) {
	return toSlice(c.conn.Exec(ctx, cmd, args))
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

func (c *Client) Int64Slice(ctx context.Context, cmd string, args ...interface{}) ([]int64, error) {
	return toInt64Slice(c.conn.Exec(ctx, cmd, args))
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

func (c *Client) Float64Slice(ctx context.Context, cmd string, args ...interface{}) ([]float64, error) {
	return toFloat64Slice(c.conn.Exec(ctx, cmd, args))
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

func (c *Client) StringSlice(ctx context.Context, cmd string, args ...interface{}) ([]string, error) {
	return toStringSlice(c.conn.Exec(ctx, cmd, args))
}

func toStringMap(v interface{}, err error) (map[string]string, error) {
	if err != nil {
		return nil, err
	}
	slice, err := toStringSlice(v, err)
	if err != nil {
		return nil, err
	}
	val := make(map[string]string, len(slice)/2)
	for i := 0; i < len(slice); i += 2 {
		val[slice[i]] = slice[i+1]
	}
	return val, nil
}

func (c *Client) StringMap(ctx context.Context, cmd string, args ...interface{}) (map[string]string, error) {
	return toStringMap(c.conn.Exec(ctx, cmd, args))
}

func toZItemSlice(v interface{}, err error) ([]ZItem, error) {
	if err != nil {
		return nil, err
	}
	slice, err := toStringSlice(v, err)
	if err != nil {
		return nil, err
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

func (c *Client) ZItemSlice(ctx context.Context, cmd string, args ...interface{}) ([]ZItem, error) {
	return toZItemSlice(c.conn.Exec(ctx, cmd, args))
}
