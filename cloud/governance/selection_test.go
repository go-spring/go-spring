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

package governance

import (
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
)

// selectionPool builds a pool over a one-endpoint static source — enough for the
// provider tests, which assert the policy that reaches the pool rather than a
// routing decision. It mirrors what a client starter builds: a suspension tracker
// attached so the thresholds have somewhere to land.
func selectionPool() *loadbalance.Pool {
	src := loadbalance.SourceFunc(func() ([]discovery.Endpoint, error) {
		return []discovery.Endpoint{{Addr: "10.0.0.1:8080", Healthy: true, Weight: 1}}, nil
	})
	bal, err := loadbalance.New(loadbalance.RoundRobin)
	if err != nil {
		panic(err)
	}
	return loadbalance.NewPool(src, bal, loadbalance.WithTracker(loadbalance.NewTracker(loadbalance.TrackerConfig{})))
}

// selectionConfig returns a Config with one rule for label naming a strategy and
// an outlier-suspension policy.
func selectionConfig(label, balancer string, threshold int, suspendFor time.Duration) Config {
	return Config{
		Enabled: true,
		Rules: []Rule{{
			Resources: []string{label},
			PolicyConfig: resilience.PolicyConfig{
				Balancer:          balancer,
				OutlierThreshold:  threshold,
				OutlierSuspendFor: suspendFor,
			},
		}},
	}
}

// TestSelectionFor_AppliesCurrentThenChanges is the end-to-end control-plane test:
// the label's rule reaches the pool at bind time (before the first Pick, not only
// on the next push), and a later config refresh reaches it again — which is what
// makes endpoint selection a hot-reloadable knob rather than a construction-time
// constant.
func TestSelectionFor_AppliesCurrentThenChanges(t *testing.T) {
	const label = "http:user-svc"
	c := newCenter(selectionConfig(label, loadbalance.LeastConn, 5, 10*time.Second))
	pool := selectionPool()

	stop := c.selectionFor(label, func(s loadbalance.Selection) {
		pool.ApplySelection(s.Balancer, s.OutlierThreshold, s.OutlierSuspendFor)
	})
	defer stop()

	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)
	assert.Number(t, pool.Selection().OutlierThreshold).Equal(5)
	assert.That(t, pool.Selection().OutlierSuspendFor).Equal(10 * time.Second)
	// The suspension half landed on the tracker too, not just on the record.
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(5)

	// A pushed change reaches the live pool in place.
	c.refresh(selectionConfig(label, loadbalance.Weighted, 3, time.Minute))
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.Weighted)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(3)
}

// TestSelectionFor_UnknownBalancerKeepsLastGood pins the degrade-don't-fail rule
// at the control-plane level: a rule naming a strategy that is not registered
// leaves the last good strategy in force instead of taking the client down.
func TestSelectionFor_UnknownBalancerKeepsLastGood(t *testing.T) {
	const label = "redis:cache"
	c := newCenter(selectionConfig(label, loadbalance.LeastConn, 0, 0))
	pool := selectionPool()

	stop := c.selectionFor(label, func(s loadbalance.Selection) {
		pool.ApplySelection(s.Balancer, s.OutlierThreshold, s.OutlierSuspendFor)
	})
	defer stop()
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)

	c.refresh(selectionConfig(label, "no_such_strategy", 0, 0))
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)
}

// TestSelectionFor_DisabledIsPassThrough covers the process that imports the
// starter but has governance turned off: the pool is left exactly as built — no
// override, no suspension — instead of being armed with a partial policy.
func TestSelectionFor_DisabledIsPassThrough(t *testing.T) {
	const label = "gorm:mysql:orders-db"
	cfg := selectionConfig(label, loadbalance.LeastConn, 5, time.Second)
	cfg.Enabled = false
	c := newCenter(cfg)
	pool := selectionPool()

	stop := c.selectionFor(label, func(s loadbalance.Selection) {
		pool.ApplySelection(s.Balancer, s.OutlierThreshold, s.OutlierSuspendFor)
	})
	defer stop()

	assert.That(t, pool.Selection().Balancer).Equal("")
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(0)
}

// TestSelectionFor_StopDetaches covers the teardown half: a pool that goes away
// must be able to detach, and detaching twice must be harmless.
func TestSelectionFor_StopDetaches(t *testing.T) {
	const label = "mongodb:orders"
	c := newCenter(selectionConfig(label, loadbalance.LeastConn, 0, 0))
	pool := selectionPool()

	stop := c.selectionFor(label, func(s loadbalance.Selection) {
		pool.ApplySelection(s.Balancer, s.OutlierThreshold, s.OutlierSuspendFor)
	})
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)

	stop()
	stop()

	c.refresh(selectionConfig(label, loadbalance.Weighted, 0, 0))
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)
}

// TestGoLive_RegistersSelectionProvider covers the wiring this whole feature hangs
// on: GoLive installs the center as the process-wide selection provider, so a
// client starter that only calls loadbalance.Pool.BindSelection — without naming
// cloud/governance — gets its policy. It also pins the label contract: one rule
// drives both the protection executor and endpoint selection, because both are
// addressed by the same resource label.
func TestGoLive_RegistersSelectionProvider(t *testing.T) {
	const label = "grpc:client"
	defer Reset()
	defer loadbalance.RegisterSelectionProvider(nil)

	SetSource(NewPushSource(selectionConfig(label, loadbalance.P2C, 4, 2*time.Second)))
	GoLive()

	pool := selectionPool()
	stop := pool.BindSelection(label)
	defer stop()

	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.P2C)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(4)
}
