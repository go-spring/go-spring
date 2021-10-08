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

package testcases

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/redis"
)

func Append(t *testing.T, ctx context.Context, c redis.Client) {

	exists, err := c.Exists(ctx, "mykey")
	if err != nil {
		t.Fatal()
	}
	assert.False(t, exists)

	count, err := c.Append(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, count, int64(5))

	count, err = c.Append(ctx, "mykey", " World")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, count, int64(11))

	str, err := c.Get(ctx, "mykey")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, str, "Hello World")
}

//DECR
//redis> SET mykey "10"
//"OK"
//redis> DECR mykey
//(integer) 9
//redis> SET mykey "234293482390480948029348230948"
//"OK"
//redis> DECR mykey
//ERR ERR value is not an integer or out of range
//redis>

//DECRBY
//redis> SET mykey "10"
//"OK"
//redis> DECRBY mykey 3
//(integer) 7
//redis>

//GET
//redis> GET nonexisting
//(nil)
//redis> SET mykey "Hello"
//"OK"
//redis> GET mykey
//"Hello"
//redis>

//GETDEL
//redis> SET mykey "Hello"
//"OK"
//redis> GETDEL mykey
//"Hello"
//redis> GET mykey
//(nil)
//redis>

//GETRANGE
//redis> SET mykey "This is a string"
//"OK"
//redis> GETRANGE mykey 0 3
//"This"
//redis> GETRANGE mykey -3 -1
//"ing"
//redis> GETRANGE mykey 0 -1
//"This is a string"
//redis> GETRANGE mykey 10 100
//"string"
//redis>

//GETSET
//redis> INCR mycounter
//(integer) 1
//redis> GETSET mycounter "0"
//"1"
//redis> GET mycounter
//"0"
//redis>

//GETSET
//redis> SET mykey "Hello"
//"OK"
//redis> GETSET mykey "World"
//"Hello"
//redis> GET mykey
//"World"
//redis>

//INCR
//redis> SET mykey "10"
//"OK"
//redis> INCR mykey
//(integer) 11
//redis> GET mykey
//"11"
//redis>

//INCRBY
//redis> SET mykey "10"
//"OK"
//redis> INCRBY mykey 5
//(integer) 15
//redis>

//INCRBYFLOAT
//redis> SET mykey 10.50
//"OK"
//redis> INCRBYFLOAT mykey 0.1
//"10.6"
//redis> INCRBYFLOAT mykey -5
//"5.6"
//redis> SET mykey 5.0e3
//"OK"
//redis> INCRBYFLOAT mykey 2.0e2
//"5200"
//redis>

//MGET
//redis> SET key1 "Hello"
//"OK"
//redis> SET key2 "World"
//"OK"
//redis> MGET key1 key2 nonexisting
//1) "Hello"
//2) "World"
//3) (nil)
//redis>

//MSET
//redis> MSET key1 "Hello" key2 "World"
//"OK"
//redis> GET key1
//"Hello"
//redis> GET key2
//"World"
//redis>

//MSETNX
//redis> MSETNX key1 "Hello" key2 "there"
//(integer) 1
//redis> MSETNX key2 "new" key3 "world"
//(integer) 0
//redis> MGET key1 key2 key3
//1) "Hello"
//2) "there"
//3) (nil)
//redis>

//PSETEX
//redis> PSETEX mykey 1000 "Hello"
//"OK"
//redis> PTTL mykey
//(integer) 999
//redis> GET mykey
//"Hello"
//redis>

//SET
//redis> SET mykey "Hello"
//"OK"
//redis> GET mykey
//"Hello"
//redis> SET anotherkey "will expire in a minute" EX 60
//"OK"
//redis>

//SETEX
//redis> SETEX mykey 10 "Hello"
//"OK"
//redis> TTL mykey
//(integer) 10
//redis> GET mykey
//"Hello"
//redis>

//SETNX
//redis> SETNX mykey "Hello"
//(integer) 1
//redis> SETNX mykey "World"
//(integer) 0
//redis> GET mykey
//"Hello"
//redis>

//SETRANGE
//redis> SET key1 "Hello World"
//"OK"
//redis> SETRANGE key1 6 "Redis"
//(integer) 11
//redis> GET key1
//"Hello Redis"
//redis>

//SETRANGE
//redis> SETRANGE key2 6 "Redis"
//(integer) 11
//redis> GET key2
//"\u0000\u0000\u0000\u0000\u0000\u0000Redis"
//redis>

//STRLEN
//redis> SET mykey "Hello world"
//"OK"
//redis> STRLEN mykey
//(integer) 11
//redis> STRLEN nonexisting
//(integer) 0
//redis>
