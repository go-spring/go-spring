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

package StarterRegistryZookeeper

import (
	"encoding/json"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestInstanceID(t *testing.T) {
	// An explicit ID is used verbatim.
	assert.That(t, instanceID(instance{ID: "fixed", ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("fixed")
	// Otherwise it is derived from name and addr so restarts replace the entry.
	assert.That(t, instanceID(instance{ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("orders-1.2.3.4:80")
}

func TestPathFor(t *testing.T) {
	// The base path's trailing slash is normalised away at construction, so the
	// znode path has exactly one separator per level.
	r := &zkRegistrar{basePath: "/services"}
	got := r.pathFor(instance{ServiceName: "orders", Addr: "1.2.3.4:80"})
	assert.That(t, got).Equal("/services/orders/orders-1.2.3.4:80")
}

func TestNormalizeWeight(t *testing.T) {
	// A misconfigured negative weight clamps to 1.
	assert.That(t, normalizeWeight(-5)).Equal(1)
	// 0 is the drain signal and passes through untouched, on both write paths.
	assert.That(t, normalizeWeight(0)).Equal(0)
	// An explicit positive weight passes through unchanged.
	assert.That(t, normalizeWeight(100)).Equal(100)
}

func TestInstanceValueDrainEncoding(t *testing.T) {
	// The drain signal (weight 0) serializes as an omitted weight field —
	// a reader reconstructs 0 and excludes the instance from picking.
	b, err := json.Marshal(instanceValue{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 0})
	assert.Error(t, err).Nil()
	assert.String(t, string(b)).Equal(`{"service_name":"orders","addr":"1.2.3.4:80"}`)

	var v instanceValue
	assert.Error(t, json.Unmarshal(b, &v)).Nil()
	assert.Number(t, v.Weight).Equal(0)
}
