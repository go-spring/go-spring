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
	"sort"
	"testing"

	"github.com/go-spring/spring-base/assert"
)

func (c *Cases) SAdd() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "myset", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "myset", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "myset", "World")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(0))

			r4, err := c.SMembers(ctx, "myset")
			assert.Nil(t, err)
			sort.Strings(r4)
			assert.Equal(t, r4, []string{"Hello", "World"})
		},
		Skip: true,
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD myset Hello",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset World",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset World",
				"Response": "\"0\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMEMBERS myset",
				"Response": "\"Hello\",\"World\""
			}]
		}`,
	}
}

func (c *Cases) SCard() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "myset", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "myset", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SCard(ctx, "myset")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(2))
		},
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD myset Hello",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset World",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SCARD myset",
				"Response": "\"2\""
			}]
		}`,
	}
}
func (c *Cases) SDiff() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "key1", "a")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "key1", "b")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "key1", "c")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.SAdd(ctx, "key2", "c")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(1))

			r5, err := c.SAdd(ctx, "key2", "d")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(1))

			r6, err := c.SAdd(ctx, "key2", "e")
			assert.Nil(t, err)
			assert.Equal(t, r6, int64(1))

			r7, err := c.SDiff(ctx, "key1", "key2")
			assert.Nil(t, err)
			sort.Strings(r7)
			assert.Equal(t, r7, []string{"a", "b"})
		},
		Skip: true,
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD key1 a",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 b",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 d",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 e",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SDIFF key1 key2",
				"Response": "\"a\",\"b\""
			}]
		}`,
	}
}

func (c *Cases) SDiffStore() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "key1", "a")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "key1", "b")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "key1", "c")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.SAdd(ctx, "key2", "c")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(1))

			r5, err := c.SAdd(ctx, "key2", "d")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(1))

			r6, err := c.SAdd(ctx, "key2", "e")
			assert.Nil(t, err)
			assert.Equal(t, r6, int64(1))

			r7, err := c.SDiffStore(ctx, "key", "key1", "key2")
			assert.Nil(t, err)
			assert.Equal(t, r7, int64(2))

			r8, err := c.SMembers(ctx, "key")
			assert.Nil(t, err)
			sort.Strings(r8)
			assert.Equal(t, r8, []string{"a", "b"})
		},
		Skip: true,
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD key1 a",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 b",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 d",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 e",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SDIFFSTORE key key1 key2",
				"Response": "\"2\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMEMBERS key",
				"Response": "\"a\",\"b\""
			}]
		}`,
	}
}

func (c *Cases) SInter() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "key1", "a")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "key1", "b")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "key1", "c")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.SAdd(ctx, "key2", "c")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(1))

			r5, err := c.SAdd(ctx, "key2", "d")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(1))

			r6, err := c.SAdd(ctx, "key2", "e")
			assert.Nil(t, err)
			assert.Equal(t, r6, int64(1))

			r7, err := c.SInter(ctx, "key1", "key2")
			assert.Nil(t, err)
			assert.Equal(t, r7, []string{"c"})
		},
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD key1 a",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 b",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 d",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 e",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SINTER key1 key2",
				"Response": "\"c\""
			}]
		}`,
	}
}

func (c *Cases) SInterStore() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "key1", "a")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "key1", "b")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "key1", "c")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.SAdd(ctx, "key2", "c")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(1))

			r5, err := c.SAdd(ctx, "key2", "d")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(1))

			r6, err := c.SAdd(ctx, "key2", "e")
			assert.Nil(t, err)
			assert.Equal(t, r6, int64(1))

			r7, err := c.SInterStore(ctx, "key", "key1", "key2")
			assert.Nil(t, err)
			assert.Equal(t, r7, int64(1))

			r8, err := c.SMembers(ctx, "key")
			assert.Nil(t, err)
			assert.Equal(t, r8, []string{"c"})
		},
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD key1 a",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 b",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 d",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 e",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SINTERSTORE key key1 key2",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMEMBERS key",
				"Response": "\"c\""
			}]
		}`,
	}
}

func (c *Cases) SIsMember() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "myset", "one")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SIsMember(ctx, "myset", "one")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SIsMember(ctx, "myset", "two")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(0))
		},
		Data: "",
	}
}

func (c *Cases) SMembers() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "myset", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "myset", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SMembers(ctx, "myset")
			assert.Nil(t, err)
			sort.Strings(r3)
			assert.Equal(t, r3, []string{"Hello", "World"})
		},
		Skip: true,
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD myset Hello",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset World",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMEMBERS myset",
				"Response": "\"Hello\",\"World\""
			}]
		}`,
	}
}

func (c *Cases) SMIsMember() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "myset", "one")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "myset", "one")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(0))

			r3, err := c.SMIsMember(ctx, "myset", "one", "notamember")
			assert.Nil(t, err)
			assert.Equal(t, r3, []int64{int64(1), int64(0)})
		},
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD myset one",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset one",
				"Response": "\"0\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMISMEMBER myset one notamember",
				"Response": "\"1\",\"0\""
			}]
		}`,
	}
}

func (c *Cases) SMove() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "myset", "one")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "myset", "two")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "myotherset", "three")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.SMove(ctx, "myset", "myotherset", "two")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(1))

			r5, err := c.SMembers(ctx, "myset")
			assert.Nil(t, err)
			assert.Equal(t, r5, []string{"one"})

			r6, err := c.SMembers(ctx, "myotherset")
			assert.Nil(t, err)
			sort.Strings(r6)
			assert.Equal(t, r6, []string{"three", "two"})
		},
		Skip: true,
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD myset one",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset two",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myotherset three",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMOVE myset myotherset two",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMEMBERS myset",
				"Response": "\"one\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMEMBERS myotherset",
				"Response": "\"two\",\"three\""
			}]
		}`,
	}
}

func (c *Cases) SPop() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "myset", "one")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "myset", "two")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "myset", "three")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.SPop(ctx, "myset")
			assert.Nil(t, err)

			r5, err := c.SMembers(ctx, "myset")
			assert.Nil(t, err)

			r6 := append([]string{r4}, r5...)
			sort.Strings(r6)
			assert.Equal(t, r6, []string{"one", "three", "two"})
		},
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD myset one",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset two",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset three",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SPOP myset",
				"Response": "\"two\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMEMBERS myset",
				"Response": "\"three\",\"one\""
			}]
		}`,
	}
}

func (c *Cases) SRandMember() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "myset", "one", "two", "three")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(3))

			_, err = c.SRandMember(ctx, "myset")
			assert.Nil(t, err)

			r3, err := c.SRandMemberN(ctx, "myset", 2)
			assert.Nil(t, err)
			assert.Equal(t, len(r3), 2)

			r4, err := c.SRandMemberN(ctx, "myset", -5)
			assert.Nil(t, err)
			assert.Equal(t, len(r4), 5)
		},
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD myset one two three",
				"Response": "\"3\""
			}, {
				"Protocol": "REDIS",
				"Request": "SRANDMEMBER myset",
				"Response": "\"one\""
			}, {
				"Protocol": "REDIS",
				"Request": "SRANDMEMBER myset 2",
				"Response": "\"one\",\"three\""
			}, {
				"Protocol": "REDIS",
				"Request": "SRANDMEMBER myset -5",
				"Response": "\"one\",\"one\",\"one\",\"two\",\"one\""
			}]
		}`,
	}
}

func (c *Cases) SRem() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "myset", "one")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "myset", "two")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "myset", "three")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.SRem(ctx, "myset", "one")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(1))

			r5, err := c.SRem(ctx, "myset", "four")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(0))

			r6, err := c.SMembers(ctx, "myset")
			assert.Nil(t, err)
			sort.Strings(r6)
			assert.Equal(t, r6, []string{"three", "two"})
		},
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD myset one",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset two",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD myset three",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SREM myset one",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SREM myset four",
				"Response": "\"0\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMEMBERS myset",
				"Response": "\"three\",\"two\""
			}]
		}`,
	}
}

func (c *Cases) SUnion() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "key1", "a")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "key1", "b")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "key1", "c")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.SAdd(ctx, "key2", "c")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(1))

			r5, err := c.SAdd(ctx, "key2", "d")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(1))

			r6, err := c.SAdd(ctx, "key2", "e")
			assert.Nil(t, err)
			assert.Equal(t, r6, int64(1))

			r7, err := c.SUnion(ctx, "key1", "key2")
			assert.Nil(t, err)
			sort.Strings(r7)
			assert.Equal(t, r7, []string{"a", "b", "c", "d", "e"})
		},
		Skip: true,
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD key1 a",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 b",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 d",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 e",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SUNION key1 key2",
				"Response": "\"a\",\"b\",\"c\",\"d\",\"e\""
			}]
		}`,
	}
}

func (c *Cases) SUnionStore() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SAdd(ctx, "key1", "a")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SAdd(ctx, "key1", "b")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "key1", "c")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.SAdd(ctx, "key2", "c")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(1))

			r5, err := c.SAdd(ctx, "key2", "d")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(1))

			r6, err := c.SAdd(ctx, "key2", "e")
			assert.Nil(t, err)
			assert.Equal(t, r6, int64(1))

			r7, err := c.SUnionStore(ctx, "key", "key1", "key2")
			assert.Nil(t, err)
			assert.Equal(t, r7, int64(5))

			r8, err := c.SMembers(ctx, "key")
			assert.Nil(t, err)
			sort.Strings(r8)
			assert.Equal(t, r8, []string{"a", "b", "c", "d", "e"})
		},
		Skip: true,
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SADD key1 a",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 b",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key1 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 c",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 d",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key2 e",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SUNIONSTORE key key1 key2",
				"Response": "\"5\""
			}, {
				"Protocol": "REDIS",
				"Request": "SMEMBERS key",
				"Response": "\"a\",\"b\",\"c\",\"d\",\"e\""
			}]
		}`,
	}
}
