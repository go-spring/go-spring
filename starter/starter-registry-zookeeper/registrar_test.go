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
	"testing"

	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestInstanceID(t *testing.T) {
	// An explicit ID is used verbatim.
	assert.That(t, instanceID(discovery.Registration{ID: "fixed", ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("fixed")
	// Otherwise it is derived from name and addr so restarts replace the entry.
	assert.That(t, instanceID(discovery.Registration{ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("orders-1.2.3.4:80")
}

func TestPathFor(t *testing.T) {
	// The base path's trailing slash is normalised away at construction, so the
	// znode path has exactly one separator per level.
	r := &zkRegistrar{basePath: "/services"}
	got := r.pathFor(discovery.Registration{ServiceName: "orders", Addr: "1.2.3.4:80"})
	assert.That(t, got).Equal("/services/orders/orders-1.2.3.4:80")
}
