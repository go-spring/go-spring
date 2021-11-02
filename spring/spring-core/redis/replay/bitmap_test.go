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

package replay

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-core/redis"
	"github.com/go-spring/spring-core/redis/testcases"
)

func TestBitCount(t *testing.T) {
	runTest(t, testcases.BitCount, func() []*fastdev.Action {
		return []*fastdev.Action{
			{
				Protocol: fastdev.REDIS,
				Request:  "SET mykey foobar",
				Response: "OK",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "BITCOUNT mykey",
				Response: "26",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "BITCOUNT mykey 0 0",
				Response: "4",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "BITCOUNT mykey 1 1",
				Response: "6",
			},
		}
	})
}

func TestBitOpAnd(t *testing.T) {
	runTest(t, testcases.BitOpAnd, func() []*fastdev.Action {
		return []*fastdev.Action{
			{
				Protocol: fastdev.REDIS,
				Request:  "SET key1 foobar",
				Response: "OK",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "SET key2 abcdef",
				Response: "OK",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "BITOP AND dest key1 key2",
				Response: "6",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "GET dest",
				Response: "`bc`ab",
			},
		}
	})
}

func BitPos(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "\xff\xf0\x00")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.BitPos(ctx, "mykey", 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(12))

	r3, err := c.Set(ctx, "mykey", "\x00\xff\xf0")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, true)

	r4, err := c.BitPos(ctx, "mykey", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(8))

	r5, err := c.BitPos(ctx, "mykey", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(16))

	r6, err := c.Set(ctx, "mykey", "\x00\x00\x00")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r6, true)

	r7, err := c.BitPos(ctx, "mykey", 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r7, int64(-1))
}

func TestGetBit(t *testing.T) {
	runTest(t, testcases.GetBit, func() []*fastdev.Action {
		return []*fastdev.Action{
			{
				Protocol: fastdev.REDIS,
				Request:  "SETBIT mykey 7 1",
				Response: "0",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "GETBIT mykey 0",
				Response: "0",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "GETBIT mykey 7",
				Response: "1",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "GETBIT mykey 100",
				Response: "0",
			},
		}
	})
}

func TestSetBit(t *testing.T) {
	runTest(t, testcases.SetBit, func() []*fastdev.Action {
		return []*fastdev.Action{
			{
				Protocol: fastdev.REDIS,
				Request:  "SETBIT mykey 7 1",
				Response: "0",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "SETBIT mykey 7 0",
				Response: "1",
			},
			{
				Protocol: fastdev.REDIS,
				Request:  "GET mykey",
				Response: "\u0000",
			},
		}
	})
}
