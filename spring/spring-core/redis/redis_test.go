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

package redis_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/run"
	"github.com/go-spring/spring-core/redis"
	"github.com/golang/mock/gomock"
)

type mockCase func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver)

func runMockCase(t *testing.T, f mockCase) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock := redis.NewMockDriver(ctrl)
	client := redis.NewClient(mock)
	ctx := context.Background()
	f(t, ctx, client, mock)
}

type skipDriver struct {
	next redis.Driver
	skip bool
}

func (d *skipDriver) Exec(ctx context.Context, args []interface{}) (interface{}, error) {
	if d.skip {
		return nil, nil
	}
	return d.next.Exec(ctx, args)
}

func TestRecord(t *testing.T) {
	reset := run.SetMode(run.RecordModeFlag)
	defer func() {
		reset()
		redis.Recorder = nil
	}()
	var d *skipDriver
	redis.Recorder = func(next redis.Driver) redis.Driver {
		if d == nil {
			d = &skipDriver{
				next: next,
				skip: false,
			}
		}
		return d
	}
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return(int64(3), nil)
		i, err := client.Append(ctx, "mykey", "abc")
		assert.Nil(t, err)
		assert.Equal(t, i, int64(3))
	})
	d.skip = true
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		_, _ = client.Append(ctx, "mykey", "abc")
	})
}

func TestReplay(t *testing.T) {
	reset := run.SetMode(run.ReplayModeFlag)
	defer func() {
		reset()
		redis.Replayer = nil
	}()
	var d *skipDriver
	redis.Replayer = func(next redis.Driver) redis.Driver {
		if d == nil {
			d = &skipDriver{
				next: next,
				skip: false,
			}
		}
		return d
	}
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return(int64(3), nil)
		i, err := client.Append(ctx, "mykey", "abc")
		assert.Nil(t, err)
		assert.Equal(t, i, int64(3))
	})
	d.skip = true
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		_, _ = client.Append(ctx, "mykey", "abc")
	})
}

func TestToInt64(t *testing.T) {
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return(nil, errors.New("abc"))
		_, err := client.Append(ctx, "mykey", "abc")
		assert.Equal(t, err, errors.New("abc"))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return(nil, nil)
		i, err := client.Append(ctx, "mykey", "abc")
		assert.Nil(t, err)
		assert.Equal(t, i, int64(0))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return(int64(3), nil)
		i, err := client.Append(ctx, "mykey", "abc")
		assert.Nil(t, err)
		assert.Equal(t, i, int64(3))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return(float64(3), nil)
		i, err := client.Append(ctx, "mykey", "abc")
		assert.Nil(t, err)
		assert.Equal(t, i, int64(3))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return("3", nil)
		i, err := client.Append(ctx, "mykey", "abc")
		assert.Nil(t, err)
		assert.Equal(t, i, int64(3))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return(redis.NewResult("3"), nil)
		i, err := client.Append(ctx, "mykey", "abc")
		assert.Nil(t, err)
		assert.Equal(t, i, int64(3))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return(redis.NewResult(), nil)
		_, err := client.Append(ctx, "mykey", "abc")
		assert.Error(t, err, "redis: no data")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"APPEND", "mykey", "abc"}).Return([]string{"3"}, nil)
		_, err := client.Append(ctx, "mykey", "abc")
		assert.Error(t, err, "redis: unexpected type \\(\\[\\]string\\) for int64")
	})
}

func TestToFloat64(t *testing.T) {
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"INCRBYFLOAT", "mykey", 1.5}).Return(nil, errors.New("abc"))
		_, err := client.IncrByFloat(ctx, "mykey", 1.5)
		assert.Equal(t, err, errors.New("abc"))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"INCRBYFLOAT", "mykey", 1.5}).Return(nil, nil)
		i, err := client.IncrByFloat(ctx, "mykey", 1.5)
		assert.Nil(t, err)
		assert.Equal(t, i, float64(0))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"INCRBYFLOAT", "mykey", 1.5}).Return(5.4, nil)
		i, err := client.IncrByFloat(ctx, "mykey", 1.5)
		assert.Nil(t, err)
		assert.Equal(t, i, 5.4)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"INCRBYFLOAT", "mykey", 1.5}).Return(int64(5), nil)
		i, err := client.IncrByFloat(ctx, "mykey", 1.5)
		assert.Nil(t, err)
		assert.Equal(t, i, 5.0)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"INCRBYFLOAT", "mykey", 1.5}).Return("5", nil)
		i, err := client.IncrByFloat(ctx, "mykey", 1.5)
		assert.Nil(t, err)
		assert.Equal(t, i, 5.0)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"INCRBYFLOAT", "mykey", 1.5}).Return(redis.NewResult("5"), nil)
		i, err := client.IncrByFloat(ctx, "mykey", 1.5)
		assert.Nil(t, err)
		assert.Equal(t, i, 5.0)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"INCRBYFLOAT", "mykey", 1.5}).Return(redis.NewResult(), nil)
		_, err := client.IncrByFloat(ctx, "mykey", 1.5)
		assert.Error(t, err, "redis: no data")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"INCRBYFLOAT", "mykey", 1.5}).Return([]string{"5"}, nil)
		_, err := client.IncrByFloat(ctx, "mykey", 1.5)
		assert.Error(t, err, "redis: unexpected type \\(\\[\\]string\\) for float64")
	})
}

func TestToString(t *testing.T) {
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"GET", "mykey"}).Return(nil, errors.New("abc"))
		_, err := client.Get(ctx, "mykey")
		assert.Equal(t, err, errors.New("abc"))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"GET", "mykey"}).Return(nil, nil)
		i, err := client.Get(ctx, "mykey")
		assert.Nil(t, err)
		assert.Equal(t, i, "")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"GET", "mykey"}).Return("abc", nil)
		i, err := client.Get(ctx, "mykey")
		assert.Nil(t, err)
		assert.Equal(t, i, "abc")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"GET", "mykey"}).Return(redis.NewResult("abc"), nil)
		i, err := client.Get(ctx, "mykey")
		assert.Nil(t, err)
		assert.Equal(t, i, "abc")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"GET", "mykey"}).Return(redis.NewResult(), nil)
		_, err := client.Get(ctx, "mykey")
		assert.Error(t, err, "redis: no data")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"GET", "mykey"}).Return(3, nil)
		_, err := client.Get(ctx, "mykey")
		assert.Error(t, err, "redis: unexpected type \\(int\\) for string")
	})
}

func TestToSlice(t *testing.T) {
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"MGET", "mykey1", "mykey2", "mykey3"}).Return(nil, errors.New("abc"))
		_, err := client.MGet(ctx, "mykey1", "mykey2", "mykey3")
		assert.Equal(t, err, errors.New("abc"))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"MGET", "mykey1", "mykey2", "mykey3"}).Return(nil, nil)
		i, err := client.MGet(ctx, "mykey1", "mykey2", "mykey3")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"MGET", "mykey1", "mykey2", "mykey3"}).
			Return([]interface{}{"1", nil, "2"}, nil)
		i, err := client.MGet(ctx, "mykey1", "mykey2", "mykey3")
		assert.Nil(t, err)
		assert.Equal(t, i, []interface{}{"1", nil, "2"})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"MGET", "mykey1", "mykey2", "mykey3"}).
			Return([]string{"1", "NULL", "2"}, nil)
		i, err := client.MGet(ctx, "mykey1", "mykey2", "mykey3")
		assert.Nil(t, err)
		assert.Equal(t, i, []interface{}{"1", nil, "2"})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"MGET", "mykey1", "mykey2", "mykey3"}).Return([]string{}, nil)
		i, err := client.MGet(ctx, "mykey1", "mykey2", "mykey3")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"MGET", "mykey1", "mykey2", "mykey3"}).
			Return(redis.NewResult("1", "NULL", "2"), nil)
		i, err := client.MGet(ctx, "mykey1", "mykey2", "mykey3")
		assert.Nil(t, err)
		assert.Equal(t, i, []interface{}{"1", nil, "2"})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"MGET", "mykey1", "mykey2", "mykey3"}).Return(3, nil)
		_, err := client.MGet(ctx, "mykey1", "mykey2", "mykey3")
		assert.Error(t, err, "redis: unexpected type \\(int\\) for \\[\\]interface{}")
	})
}

func TestToInt64Slice(t *testing.T) {
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"LPOS", "mykey", "abc", "COUNT", int64(3)}).Return(nil, errors.New("abc"))
		_, err := client.LPosN(ctx, "mykey", "abc", 3)
		assert.Equal(t, err, errors.New("abc"))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"LPOS", "mykey", "abc", "COUNT", int64(3)}).Return(nil, nil)
		i, err := client.LPosN(ctx, "mykey", "abc", 3)
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"LPOS", "mykey", "abc", "COUNT", int64(3)}).
			Return([]interface{}{"1", nil, "2"}, nil)
		i, err := client.LPosN(ctx, "mykey", "abc", 3)
		assert.Nil(t, err)
		assert.Equal(t, i, []int64{1, 0, 2})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"LPOS", "mykey", "abc", "COUNT", int64(3)}).
			Return([]string{"1", "NULL", "2"}, nil)
		i, err := client.LPosN(ctx, "mykey", "abc", 3)
		assert.Nil(t, err)
		assert.Equal(t, i, []int64{1, 0, 2})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"LPOS", "mykey", "abc", "COUNT", int64(3)}).Return([]string{}, nil)
		i, err := client.LPosN(ctx, "mykey", "abc", 3)
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"LPOS", "mykey", "abc", "COUNT", int64(3)}).
			Return(redis.NewResult("1", "NULL", "2"), nil)
		i, err := client.LPosN(ctx, "mykey", "abc", 3)
		assert.Nil(t, err)
		assert.Equal(t, i, []int64{1, 0, 2})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"LPOS", "mykey", "abc", "COUNT", int64(3)}).Return(3, nil)
		_, err := client.LPosN(ctx, "mykey", "abc", 3)
		assert.Error(t, err, "redis: unexpected type \\(int\\) for \\[\\]interface{}")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"LPOS", "mykey", "abc", "COUNT", int64(3)}).
			Return([]string{"abc"}, nil)
		_, err := client.LPosN(ctx, "mykey", "abc", 3)
		assert.Error(t, err, "strconv.ParseInt: parsing \\\"abc\\\": invalid syntax")
	})
}

func TestToFloat64Slice(t *testing.T) {
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZMSCORE", "mykey", "one", "two"}).Return(nil, errors.New("abc"))
		_, err := client.ZMScore(ctx, "mykey", "one", "two")
		assert.Equal(t, err, errors.New("abc"))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZMSCORE", "mykey", "one", "two"}).Return(nil, nil)
		i, err := client.ZMScore(ctx, "mykey", "one", "two")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZMSCORE", "mykey", "one", "two"}).
			Return([]interface{}{"1", nil, "2"}, nil)
		i, err := client.ZMScore(ctx, "mykey", "one", "two")
		assert.Nil(t, err)
		assert.Equal(t, i, []float64{1, 0, 2})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZMSCORE", "mykey", "one", "two"}).
			Return([]string{"1", "NULL", "2"}, nil)
		i, err := client.ZMScore(ctx, "mykey", "one", "two")
		assert.Nil(t, err)
		assert.Equal(t, i, []float64{1, 0, 2})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZMSCORE", "mykey", "one", "two"}).Return([]string{}, nil)
		i, err := client.ZMScore(ctx, "mykey", "one", "two")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZMSCORE", "mykey", "one", "two"}).
			Return(redis.NewResult("1", "NULL", "2"), nil)
		i, err := client.ZMScore(ctx, "mykey", "one", "two")
		assert.Nil(t, err)
		assert.Equal(t, i, []float64{1, 0, 2})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZMSCORE", "mykey", "one", "two"}).Return(3, nil)
		_, err := client.ZMScore(ctx, "mykey", "one", "two")
		assert.Error(t, err, "redis: unexpected type \\(int\\) for \\[\\]interface{}")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZMSCORE", "mykey", "one", "two"}).
			Return([]string{"abc"}, nil)
		_, err := client.ZMScore(ctx, "mykey", "one", "two")
		assert.Error(t, err, "strconv.ParseFloat: parsing \\\"abc\\\": invalid syntax")
	})
}

func TestToStringSlice(t *testing.T) {
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"KEYS", "*"}).Return(nil, errors.New("abc"))
		_, err := client.Keys(ctx, "*")
		assert.Equal(t, err, errors.New("abc"))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"KEYS", "*"}).Return(nil, nil)
		i, err := client.Keys(ctx, "*")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"KEYS", "*"}).
			Return([]interface{}{"1", nil, "2"}, nil)
		i, err := client.Keys(ctx, "*")
		assert.Nil(t, err)
		assert.Equal(t, i, []string{"1", "", "2"})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"KEYS", "*"}).
			Return([]string{"1", "NULL", "2"}, nil)
		i, err := client.Keys(ctx, "*")
		assert.Nil(t, err)
		assert.Equal(t, i, []string{"1", "", "2"})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"KEYS", "*"}).Return([]string{}, nil)
		i, err := client.Keys(ctx, "*")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"KEYS", "*"}).
			Return(redis.NewResult("1", "NULL", "2"), nil)
		i, err := client.Keys(ctx, "*")
		assert.Nil(t, err)
		assert.Equal(t, i, []string{"1", "", "2"})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"KEYS", "*"}).Return(3, nil)
		_, err := client.Keys(ctx, "*")
		assert.Error(t, err, "redis: unexpected type \\(int\\) for \\[\\]interface{}")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"KEYS", "*"}).Return([]interface{}{3}, nil)
		_, err := client.Keys(ctx, "*")
		assert.Error(t, err, "redis: unexpected type \\(int\\) for string")
	})
}

func TestToStringMap(t *testing.T) {
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"HGETALL", "mykey"}).Return(nil, errors.New("abc"))
		_, err := client.HGetAll(ctx, "mykey")
		assert.Equal(t, err, errors.New("abc"))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"HGETALL", "mykey"}).Return(nil, nil)
		i, err := client.HGetAll(ctx, "mykey")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"HGETALL", "mykey"}).
			Return([]interface{}{"1", nil, "2"}, nil)
		_, err := client.HGetAll(ctx, "mykey")
		assert.Error(t, err, "redis: unexpected slice length 3")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"HGETALL", "mykey"}).
			Return([]interface{}{"1", nil, "2", "abc"}, nil)
		i, err := client.HGetAll(ctx, "mykey")
		assert.Nil(t, err)
		assert.Equal(t, i, map[string]string{"1": "", "2": "abc"})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"HGETALL", "mykey"}).
			Return([]string{"1", "NULL", "2", "abc"}, nil)
		i, err := client.HGetAll(ctx, "mykey")
		assert.Nil(t, err)
		assert.Equal(t, i, map[string]string{"1": "", "2": "abc"})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"HGETALL", "mykey"}).Return([]string{}, nil)
		i, err := client.HGetAll(ctx, "mykey")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"HGETALL", "mykey"}).
			Return(redis.NewResult("1", "NULL", "2", "abc"), nil)
		i, err := client.HGetAll(ctx, "mykey")
		assert.Nil(t, err)
		assert.Equal(t, i, map[string]string{"1": "", "2": "abc"})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"HGETALL", "mykey"}).Return(3, nil)
		_, err := client.HGetAll(ctx, "mykey")
		assert.Error(t, err, "redis: unexpected type \\(int\\) for \\[\\]interface{}")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"HGETALL", "mykey"}).Return([]interface{}{3}, nil)
		_, err := client.HGetAll(ctx, "mykey")
		assert.Error(t, err, "redis: unexpected type \\(int\\) for string")
	})
}

func TestToZItemSlice(t *testing.T) {
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).Return(nil, errors.New("abc"))
		_, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Equal(t, err, errors.New("abc"))
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).Return(nil, nil)
		i, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).
			Return([]interface{}{"1", 1.5, "2"}, nil)
		_, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Error(t, err, "redis: unexpected type \\(float64\\) for string")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).
			Return([]interface{}{"1", "1.5", "2"}, nil)
		_, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Error(t, err, "redis: unexpected slice length 3")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).
			Return([]interface{}{"1", "1.5", "2", "abc"}, nil)
		_, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Error(t, err, "strconv.ParseFloat: parsing \"abc\": invalid syntax")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).
			Return([]interface{}{"1", "1.5", "2", "3.0"}, nil)
		i, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Nil(t, err)
		assert.Equal(t, i, []redis.ZItem{{"1", 1.5}, {"2", 3.0}})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).
			Return([]string{"1", "1.5", "2", "3.0"}, nil)
		i, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Nil(t, err)
		assert.Equal(t, i, []redis.ZItem{{"1", 1.5}, {"2", 3.0}})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).Return([]string{}, nil)
		i, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Nil(t, err)
		assert.Nil(t, i)
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).
			Return(redis.NewResult("1", "1.5", "2", "3.0"), nil)
		i, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Nil(t, err)
		assert.Equal(t, i, []redis.ZItem{{"1", 1.5}, {"2", 3.0}})
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).Return(3, nil)
		_, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Error(t, err, "redis: unexpected type \\(int\\) for \\[\\]interface{}")
	})
	runMockCase(t, func(t *testing.T, ctx context.Context, client *redis.Client, mock *redis.MockDriver) {
		mock.EXPECT().Exec(ctx, []interface{}{"ZDIFF", 1, "mykey", "WITHSCORES"}).Return([]interface{}{3}, nil)
		_, err := client.ZDiffWithScores(ctx, "mykey")
		assert.Error(t, err, "redis: unexpected type \\(int\\) for string")
	})
}
