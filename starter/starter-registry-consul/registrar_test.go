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

package StarterRegistryConsul

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/spring/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestServiceID(t *testing.T) {
	// An explicit ID is used verbatim.
	assert.That(t, serviceID(discovery.Registration{ID: "fixed", ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("fixed")
	// Otherwise it is derived from name and addr so restarts replace the entry.
	assert.That(t, serviceID(discovery.Registration{ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("orders-1.2.3.4:80")
}

func TestRegister_BadAddr(t *testing.T) {
	// api.NewClient does not dial, so this needs no live agent: the addr is
	// validated before any Consul call, so a malformed addr fails fast.
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:8500"})
	assert.Error(t, err).Nil()
	r := &consulRegistrar{client: client, ttl: time.Second, heartbeats: map[string]chan struct{}{}}

	err = r.Register(context.Background(), discovery.Registration{ServiceName: "orders", Addr: "no-port"})
	assert.Error(t, err).Matches("must be host:port")

	err = r.Register(context.Background(), discovery.Registration{ServiceName: "orders", Addr: "host:abc"})
	assert.Error(t, err).Matches("non-numeric port")
}
