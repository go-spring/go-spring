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
	"go-spring.org/cloud"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// poolDriver is a stand-in for a company Driver whose topology needs a pick
// pool: it builds one, hands it to NewClient along with the governance bundle,
// and returns the wrapper — exactly as the Driver contract asks, WITHOUT any
// knowledge of governance. This test exists to pin that NewClient does the
// binding, and that the driver's only obligation is to return the pool inside
// the wrapper it builds.
type poolDriver struct {
	endpoints []discovery.Endpoint
	// returned records the pool the driver handed over, so the test can assert
	// the binding landed on THAT pool rather than on some other.
	returned *loadbalance.Pool
}

func (d *poolDriver) CreateClient(_ context.Context, c Config, params cloud.ClientParams) (*Client, error) {
	src := func() ([]discovery.Endpoint, error) { return d.endpoints, nil }
	bal := loadbalance.NewRoundRobin()
	d.returned = loadbalance.NewPool(src, bal)
	// A real *redis.Client is not needed: this driver builds no dialer.
	return NewClient(redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), c, d.returned, params)
}

// TestDriverPoolIsBoundByCaller pins the split the Driver interface encodes: the
// driver returns the wrapper holding the pool it built and says nothing about
// governance, and NewClient binds that pool to the bundle's manager. A driver
// that returns a pool must get governance for free, and one that returns none
// must not be forced to care.
func TestDriverPoolIsBoundByCaller(t *testing.T) {
	drv := &poolDriver{endpoints: []discovery.Endpoint{
		{Addr: "10.0.0.1:6379", Healthy: true},
		{Addr: "10.0.0.2:6379", Healthy: true},
	}}

	// The manager is applied to the bundle the driver receives; binding an
	// unarmed manager is safe, so the pool is subscribed and armed when the
	// manager goes live — standing in for a center that pushes later.
	mgr, err := loadbalance.NewManager(nil) // no factory bean contributed
	assert.Error(t, err).Nil()
	w, err := drv.CreateClient(context.Background(), Config{Addr: "10.0.0.1:6379"},
		cloud.ClientParams{Resilience: resilience.NewManager(nil), Loadbalance: mgr})
	assert.Error(t, err).Nil()

	// The rule gives this service a strategy the pool was not built with.
	mgr.Apply(loadbalance.Settings{Enabled: true, Resolve: func(string) loadbalance.Selection {
		return loadbalance.Selection{
			Balancer:          loadbalance.ConsistentHash,
			OutlierThreshold:  3,
			OutlierSuspendFor: time.Second,
		}
	}})

	// The binding landed on the pool the driver returned...
	assert.That(t, drv.returned).NotNil()
	assert.That(t, drv.returned.Selection().Balancer).Equal(loadbalance.ConsistentHash)
	assert.Number(t, drv.returned.Tracker().Config().Threshold).Equal(3)
	// ...and Destroy releases it rather than leaking the subscription.
	assert.Error(t, w.Destroy()).Nil()
}

// TestDriverPoolNilIsFine pins the other half: a driver whose topology needs no
// pick pool passes nil and NewClient binds nothing — no nil dereference, no
// forced governance awareness.
func TestDriverPoolNilIsFine(t *testing.T) {
	lb, err := loadbalance.NewManager(nil) // no factory bean contributed
	assert.Error(t, err).Nil()
	w, err := NewClient(redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), Config{Addr: "10.0.0.1:6379"}, nil,
		cloud.ClientParams{Resilience: resilience.NewManager(nil), Loadbalance: lb})
	assert.Error(t, err).Nil()
	assert.That(t, w.lbPool).Nil()
	assert.That(t, w.detach).Nil()
}
