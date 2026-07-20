/*
 * Copyright 2025 The Go-Spring Authors.
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

package StarterRedigo

import (
	"github.com/gomodule/redigo/redis"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/discovery"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/spring/starter"
)

func init() {
	// Register multiple Redis clients as a group.
	// Each instance is created according to the configuration in "${spring.redigo}".
	// This allows defining multiple redis instances dynamically.
	gs.Group("${spring.redigo}", newClient, destroyClient)
}

// newClient creates a new Redis client based on the provided configuration.
func newClient(c Config) (*redis.Pool, error) {
	if err := starter.RequireAny("redis",
		starter.Field{Name: "addr", Value: c.Addr},
		starter.Field{Name: "service-name", Value: c.ServiceName},
	); err != nil {
		return nil, err
	}
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "redis driver not found: %s", c.Driver)
	}
	pool, err := d.CreateClient(c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create redis client")
	}
	// Fail fast: the redigo pool dials lazily, so borrow one connection and
	// PING it at startup. A misconfigured address or unreachable server then
	// surfaces during boot rather than on the first request.
	conn := pool.Get()
	defer func() { _ = conn.Close() }()
	if _, err := conn.Do("PING"); err != nil {
		_ = pool.Close()
		return nil, errutil.Explain(err, "redis: startup ping failed")
	}
	return pool, nil
}

// destroyClient closes the Redis pool and stops any discovery watch behind it.
func destroyClient(pool *redis.Pool) error {
	if v, ok := liveDialers.LoadAndDelete(pool); ok {
		_ = v.(*discovery.LiveDialer).Stop()
	}
	return pool.Close()
}
