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
	"fmt"
	"time"

	"github.com/go-spring/spring-core/redis"
	g "github.com/gomodule/redigo/redis"
)

type Driver struct{}

func NewDriver() redis.Driver {
	return new(Driver)
}

func NewClient(config redis.Config) (redis.Client, error) {
	return redis.NewClient(config, NewDriver())
}

func (d *Driver) Open(config redis.Config) (redis.Conn, error) {

	address := fmt.Sprintf("%s:%d", config.Host, config.Port)
	conn, err := g.Dial("tcp", address,
		g.DialUsername(config.Username),
		g.DialPassword(config.Password),
		g.DialDatabase(config.Database),
		g.DialConnectTimeout(time.Duration(config.ConnectTimeout)*time.Millisecond),
		g.DialReadTimeout(time.Duration(config.ReadTimeout)*time.Millisecond),
		g.DialWriteTimeout(time.Duration(config.WriteTimeout)*time.Millisecond))
	if err != nil {
		return nil, err
	}

	if config.Ping {
		if _, err = conn.Do("PING"); err != nil {
			return nil, err
		}
	}

	return &Conn{conn: conn}, nil
}

type Conn struct {
	conn g.Conn
}

func (c *Conn) Exec(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
	result, err := c.conn.Do(cmd, args...)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, redis.ErrNil()
	}
	switch r := result.(type) {
	case []byte:
		return string(r), nil
	case []interface{}:
		for i := 0; i < len(r); i++ {
			if s, ok := r[i].([]byte); ok {
				r[i] = string(s)
			}
		}
	}
	return result, nil
}
