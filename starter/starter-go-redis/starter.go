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

package StarterGoRedis

import (
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Redis clients as a group.
	// Each instance is created according to the configuration in "${spring.go-redis}".
	// This allows defining multiple redis instances dynamically.
	gs.Group("${spring.go-redis}", newClient, destroyClient)
}

// newClient creates a new Redis client, bridged into go-spring's unified
// observability. The redisotel hooks emit client spans and connection-pool
// metrics through the OTel globals that starter-otel installs; when starter-otel
// is absent those globals are no-ops, so this stays a zero-config opt-in that
// needs no per-component adaptation.
func newClient(c Config) (*redis.Client, error) {
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "redis driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(c)
	if err != nil {
		return nil, err
	}
	if err := redisotel.InstrumentTracing(client); err != nil {
		return nil, err
	}
	if err := redisotel.InstrumentMetrics(client); err != nil {
		return nil, err
	}
	return client, nil
}

// destroyClient closes the Redis client and stops any discovery watch behind it.
func destroyClient(client *redis.Client) error {
	if v, ok := liveDialers.LoadAndDelete(client); ok {
		_ = v.(*discovery.LiveDialer).Stop()
	}
	return client.Close()
}
