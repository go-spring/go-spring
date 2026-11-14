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

package gormcore

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/capability/source"
	admissionplane "go-spring.org/cloud/governance/controlplane/admission"
	loadbalanceplane "go-spring.org/cloud/governance/controlplane/loadbalance"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/loadbalance"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/cloud/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// testCenter returns a process runtime — an unarmed resilience manager
// (observe-only pass-through) plus an injector with no rules (transparent
// pass-through) — what a hand-built client takes now that Open receives a
// *governance.Runtime rather than a bundle of authorities.
func testCenter(t *testing.T) *governance.Runtime {
	t.Helper()
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	fact2, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := resilience.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	adm, err := admission.NewManager(fact2)
	if err != nil {
		t.Fatal(err)
	}
	center := &governance.Runtime{
		Resilience:  res,
		LoadBalance: mustLbManager(t),
		Admission:   adm,
		ClientFault: clientfault.NewManager(),
		ServerFault: serverfault.NewManager(),
		Directory:   discovery.NewManager(nil),
		Traffic:     traffic.NewDefaultPropagator(traffic.DefaultBinding()),
	}
	if _, err := loadbalanceplane.NewCenter(context.Background(), center.LoadBalance, source.NewPushSource(loadbalance.Settings{})); err != nil {
		t.Fatal(err)
	}
	if _, err := admissionplane.NewCenter(context.Background(), center.Admission, source.NewPushSource(admission.Settings{})); err != nil {
		t.Fatal(err)
	}
	return center
}

// TestOpenLeavesThePoolAloneWithoutARule proves the rule is an override and not
// a replacement: with no container, the per-instance setting still decides, so
// an app that never adopted the governance rule sees no change.
func TestOpenLeavesThePoolAloneWithoutARule(t *testing.T) {
	db, err := Open(fakeDialector{}, PoolConfig{PingTimeout: time.Second, MaxOpenConns: 3}, Options{Center: testCenter(t)})
	assert.Error(t, err).Nil("open")
	defer func() { _ = db.Destroy() }()

	sqlDB, err := db.DB.DB()
	assert.Error(t, err).Nil("raw pool")
	assert.Number(t, sqlDB.Stats().MaxOpenConnections).Equal(3)
}

// mustLbManager builds the load-balancing authority the test's runtime needs.
func mustLbManager(t *testing.T) *loadbalance.Manager {
	t.Helper()
	obs, err := loadbalance.NewOTelObserver()
	if err != nil {
		t.Fatal(err)
	}
	return loadbalance.NewManager(loadbalance.NewFactory([]loadbalance.Provider{loadbalance.NewBuiltinProvider()}), obs, loadbalance.ManagerConfig{})
}
