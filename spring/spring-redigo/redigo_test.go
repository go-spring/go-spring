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

package SpringRedigo_test

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/redis"
	"github.com/go-spring/spring-redigo"
	g "github.com/gomodule/redigo/redis"
)

func getClient(t *testing.T) redis.Client {
	conn, err := g.Dial("tcp", ":6379")
	if err != nil {
		t.Fatal()
	}
	return SpringRedigo.NewClient(conn)
}

func TestClient(t *testing.T) {

	c := getClient(t)
	var reply interface{}
	ctx := context.Background()

	reply, err := c.Set(ctx, "name", "king", 0)
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, reply, "OK")

	reply, err = c.Get(ctx, "name")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, reply, "king")
}

func TestHash(t *testing.T) {

	c := getClient(t)
	var reply interface{}
	ctx := context.Background()

	reply, err := c.HSet(ctx, "hash", "name", "king")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, reply, int64(0))

	reply, err = c.HGet(ctx, "hash", "name")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, reply, "king")
}

func TestBitmap(t *testing.T) {

	c := getClient(t)
	var reply interface{}
	ctx := context.Background()

	reply, err := c.SetBit(ctx, "bitmap", 5, 1)
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, reply, int64(1))

	reply, err = c.GetBit(ctx, "bitmap", 5)
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, reply, int64(1))
}

func TestSet(t *testing.T) {

	c := getClient(t)
	var reply interface{}
	ctx := context.Background()

	reply, err := c.SAdd(ctx, "set", 1, 2, 3)
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, reply, int64(0))

	reply, err = c.SMembers(ctx, "set")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, reply, []string{"1", "2", "3"})
}
