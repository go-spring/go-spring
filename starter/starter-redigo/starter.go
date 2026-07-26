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
	"context"

	"github.com/gomodule/redigo/redis"
	"go-spring.org/log"
	"go-spring.org/spring/experimental/cloud/discovery"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

var starterTag = log.RegisterInfraTag("redigo", "")

func init() {
	// Register multiple Redis clients as a group.
	// Each instance is created according to the configuration in "${spring.redigo}".
	// This allows defining multiple redis instances dynamically.
	gs.Group("${spring.redigo}", newClient, destroyClient)
}

// newClient creates a new Redis client based on the provided configuration.
func newClient(cp *gs.ContextProvider, c Config) (*redis.Pool, error) {
	ctx := cp.Context

	log.Debugf(ctx, starterTag, "creating redigo client, addr=%s service-name=%s", c.Addr, c.ServiceName)

	if err := errutil.RequireAny("redis",
		errutil.Field{Name: "addr", Value: c.Addr},
		errutil.Field{Name: "service-name", Value: c.ServiceName},
	); err != nil {
		return nil, err
	}
	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx, starterTag, "redigo driver not found: %s", c.Driver)
		return nil, errutil.Explain(nil, "redis driver not found: %s", c.Driver)
	}
	pool, err := d.CreateClient(ctx, c)
	if err != nil {
		log.Errorf(ctx, starterTag, "redigo: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create redis client")
	}
	// Fail fast: the redigo pool dials lazily, so borrow one connection and
	// PING it at startup. A misconfigured address or unreachable server then
	// surfaces during boot rather than on the first request.
	conn := pool.Get()
	defer func() { _ = conn.Close() }()
	if _, err := conn.Do("PING"); err != nil {
		log.Errorf(ctx, starterTag, "redigo: startup ping failed: %v", err)
		_ = pool.Close()
		return nil, errutil.Explain(err, "redis: startup ping failed")
	}
	log.Infof(ctx, starterTag, "redigo client initialized, addr=%s", c.Addr)
	return pool, nil
}

// destroyClient closes the Redis pool and stops any discovery watch behind it.
func destroyClient(pool *redis.Pool) error {
	if v, ok := liveDialers.LoadAndDelete(pool); ok {
		_ = v.(*discovery.LiveDialer).Stop()
		log.Debugf(context.Background(), starterTag, "redigo client destroyed, discovery dialer stopped")
	}
	return pool.Close()
}
