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
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
)

// poolDriver is a stand-in for a company Driver whose topology needs a pick
// pool: it builds one and hands it back, exactly as the Driver contract asks,
// WITHOUT any knowledge of governance — this test exists to pin that the caller
// does the binding, and that the driver's only obligation is to return the pool.
type poolDriver struct {
	endpoints []discovery.Endpoint
	// returned records the pool the driver handed back, so the test can assert
	// the binding landed on THAT pool rather than on some other.
	returned *loadbalance.Pool
}

func (d *poolDriver) CreateClient(context.Context, Config, discovery.Discovery) (*redis.Client, *loadbalance.Pool, error) {
	src := func() ([]discovery.Endpoint, error) { return d.endpoints, nil }
	bal, err := loadbalance.New(loadbalance.RoundRobin, loadbalance.Config{})
	if err != nil {
		return nil, nil, err
	}
	d.returned = loadbalance.NewPool(src, bal)
	// A real *redis.Client is not needed: this driver builds no dialer.
	return redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), d.returned, nil
}

// TestDriverPoolIsBoundByCaller pins the split the Driver interface encodes: the
// driver returns the pool it built and says nothing about governance, and the
// Client binds that pool to the injected manager. A driver that returns a pool
// must get governance for free, and one that returns nil must not be forced to
// care.
func TestDriverPoolIsBoundByCaller(t *testing.T) {
	drv := &poolDriver{endpoints: []discovery.Endpoint{
		{Addr: "10.0.0.1:6379", Healthy: true},
		{Addr: "10.0.0.2:6379", Healthy: true},
	}}

	// The manager is armed before Init, standing in for a center that has gone
	// live; the rule gives this service a strategy the pool was not built with.
	mgr := loadbalance.NewManager()
	mgr.Apply(loadbalance.Settings{Enabled: true, Resolve: func(string) loadbalance.Selection {
		return loadbalance.Selection{
			Balancer:          loadbalance.ConsistentHash,
			OutlierThreshold:  3,
			OutlierSuspendFor: time.Second,
		}
	}})

	c := &Client{
		cfg:     Config{Addr: "10.0.0.1:6379"},
		lbMgr:   mgr,
		lbPool:  drv.returnedPool(t),
		service: "redis:test",
	}
	c.bindSelection()

	// The binding landed on the pool the driver returned...
	assert.That(t, drv.returned).NotNil()
	assert.That(t, drv.returned.Selection().Balancer).Equal(loadbalance.ConsistentHash)
	assert.Number(t, drv.returned.Tracker().Config().Threshold).Equal(3)
	// ...and Destroy releases it rather than leaking the subscription.
	c.detach()
}

// returnedPool builds the driver's pool without going through CreateClient, so
// the test can wire the Client directly.
func (d *poolDriver) returnedPool(t *testing.T) *loadbalance.Pool {
	t.Helper()
	src := func() ([]discovery.Endpoint, error) { return d.endpoints, nil }
	bal, err := loadbalance.New(loadbalance.RoundRobin, loadbalance.Config{})
	assert.Error(t, err).Nil()
	d.returned = loadbalance.NewPool(src, bal)
	return d.returned
}

// TestDriverPoolNilIsFine pins the other half: a driver whose topology needs no
// pick pool returns nil and the Client arms no selection — no nil dereference,
// no forced governance awareness.
func TestDriverPoolNilIsFine(t *testing.T) {
	c := &Client{cfg: Config{Addr: "10.0.0.1:6379"}, service: "redis:test"}
	c.bindSelection()
	assert.That(t, c.lbMgr).Nil()
	assert.That(t, c.lbPool).Nil()
}
