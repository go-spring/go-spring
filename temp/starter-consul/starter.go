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

package StarterConsul

import (
	"github.com/hashicorp/consul/api"
	"go-spring.org/spring/gs"
)

func init() {
	// Multi-instance only: bind a map of clients under "${spring.consul}" and
	// register one named *api.Client per entry, matching the client-starter
	// archetype (no default singleton). Each entry is an independent Consul
	// connection.
	gs.Group("${spring.consul}", newClient, nil)
}

// newClient creates a new Consul client based on the provided configuration.
func newClient(c Config) (*api.Client, error) {
	return api.NewClient(&api.Config{
		Address:    c.Address,
		Scheme:     c.Scheme,
		Datacenter: c.Datacenter,
		Token:      c.Token,
		Namespace:  c.Namespace,
		WaitTime:   c.WaitTime,
	})
}
