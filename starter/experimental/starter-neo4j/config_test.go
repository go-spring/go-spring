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

package StarterNeo4j

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// TestValidateSchemeWithServiceName pins the one cross-field rule: a service
// name is meaningful only under a routing scheme, because only then does the
// driver keep consulting discovery after the seed. Direct schemes stay valid
// without a service name — that is the VIP/sidecar form.
func TestValidateSchemeWithServiceName(t *testing.T) {
	cases := []struct {
		name    string
		uri     string
		service string
		wantErr bool
	}{
		{name: "routing plain", uri: "neo4j://seed:7687", service: "graph"},
		{name: "routing tls", uri: "neo4j+s://seed:7687", service: "graph"},
		{name: "routing self-signed", uri: "neo4j+ssc://seed:7687", service: "graph"},
		{name: "direct without service", uri: "bolt://10.0.0.1:7687"},
		{name: "direct tls without service", uri: "bolt+s://10.0.0.1:7687"},
		{name: "direct with service", uri: "bolt://10.0.0.1:7687", service: "graph", wantErr: true},
		{name: "direct tls with service", uri: "bolt+s://10.0.0.1:7687", service: "graph", wantErr: true},
		{name: "bad uri", uri: "://nope", service: "graph", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := (Config{
				URI:        tc.uri,
				Addressing: discovery.Addressing{ServiceName: tc.service},
			}).Validate()
			if tc.wantErr {
				assert.Error(t, err).NotNil()
				return
			}
			assert.That(t, err).Nil()
		})
	}
}

// TestResolveSeedEligibility pins the seed's selection order: disabled
// instances never receive traffic, healthy instances win, and a zero weight
// (the naming service's drain signal) is skipped unless every candidate is
// draining.
func TestResolveSeedEligibility(t *testing.T) {
	resolve := func(eps ...discovery.Endpoint) discovery.Resolver {
		return func(context.Context) ([]discovery.Endpoint, error) { return eps, nil }
	}

	addr, err := resolveSeed(context.Background(), resolve(
		discovery.Endpoint{Addr: "disabled:1", Healthy: true, Weight: 1, Disabled: true},
		discovery.Endpoint{Addr: "healthy:2", Healthy: true, Weight: 1},
	), "graph")
	assert.That(t, err).Nil()
	assert.String(t, addr).Equal("healthy:2")

	// No healthy instance: degrade to a non-disabled one rather than failing.
	addr, err = resolveSeed(context.Background(), resolve(
		discovery.Endpoint{Addr: "unhealthy:3", Weight: 1},
	), "graph")
	assert.That(t, err).Nil()
	assert.String(t, addr).Equal("unhealthy:3")

	// Draining instances are skipped while a live one exists...
	addr, err = resolveSeed(context.Background(), resolve(
		discovery.Endpoint{Addr: "draining:4", Healthy: true, Weight: 0},
		discovery.Endpoint{Addr: "live:5", Healthy: true, Weight: 1},
	), "graph")
	assert.That(t, err).Nil()
	assert.String(t, addr).Equal("live:5")

	// ...and used only when every candidate is draining.
	addr, err = resolveSeed(context.Background(), resolve(
		discovery.Endpoint{Addr: "draining:4", Healthy: true, Weight: 0},
	), "graph")
	assert.That(t, err).Nil()
	assert.String(t, addr).Equal("draining:4")

	// Nothing eligible is a construction error, never a bogus address.
	_, err = resolveSeed(context.Background(), resolve(
		discovery.Endpoint{Addr: "disabled:6", Healthy: true, Weight: 1, Disabled: true},
	), "graph")
	assert.Error(t, err).NotNil()

	_, err = resolveSeed(context.Background(), func(context.Context) ([]discovery.Endpoint, error) {
		return nil, errors.New("registry down")
	}, "graph")
	assert.Error(t, err).NotNil()
}
