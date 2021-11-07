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

package SpringRedigo

import (
	"context"

	"github.com/go-spring/spring-core/redis"
	g "github.com/gomodule/redigo/redis"
)

type client struct {
	redis.BaseClient
	conn g.Conn
}

func NewClient(conn g.Conn) redis.Client {
	c := &client{conn: conn}
	c.DoFunc = c.do
	return c
}

func (c *client) do(ctx context.Context, args ...interface{}) (interface{}, error) {
	result, err := c.conn.Do(args[0].(string), args[1:]...)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, redis.ErrNil
	}
	return transform(result), nil
}

func transform(v interface{}) interface{} {
	switch r := v.(type) {
	case []byte:
		return string(r)
	case []interface{}:
		for i := 0; i < len(r); i++ {
			if s, ok := r[i].([]byte); ok {
				r[i] = string(s)
			}
		}
	}
	return v
}
