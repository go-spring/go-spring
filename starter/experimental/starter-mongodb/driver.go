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

// driver.go is the "construction seam" concept of this starter: the discovery
// dial seam that client construction (newClient in starter.go) owns. Unlike
// go-redis/memcached there is no Driver abstraction here — the seam is the
// discovery dial: newPickPool builds a resolver-backed endpoint picker and
// starter.go feeds its picks into the per-connection dial.
package StarterMongoDB

import (
	"context"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
)

// newPickPool resolves the registered discovery backend for c into a by-name
// Resolver and wraps it in a round-robin loadbalance pool, so each new connection
// can pick a live instance from the service's current endpoint snapshot. It
// returns (nil, nil) when discovery is not in effect — service-name unset or
// mesh mode enabled (a sidecar owns discovery+LB) — in which case the caller
// dials the configured URI hosts directly. Resolver freshness lives inside the
// backend, so the pool has no resources to release and there is no Stop-half.
func newPickPool(ctx context.Context, c Config) (*loadbalance.Pool, error) {
	resolver, err := discovery.NewResolver(ctx, c.Discovery, c.ServiceName, discovery.WithScheme(c.Scheme))
	if err != nil || resolver == nil {
		return nil, err
	}
	bal, err := loadbalance.New(loadbalance.RoundRobin)
	if err != nil {
		return nil, err
	}
	return loadbalance.NewPool(loadbalance.SourceFunc(resolver), bal), nil
}
