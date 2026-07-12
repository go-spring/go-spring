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

	// Register a single default Consul client.
	// This client will only be created if the property "spring.consul.address" is set.
	// It uses the configuration tagged with "${spring.consul}".
	gs.Provide(newClient, gs.TagArg("${spring.consul}")).
		Condition(gs.OnProperty("spring.consul.address"))

	// Register multiple Consul clients as a group.
	// Each instance is created according to the configuration in "${spring.consul.instances}".
	// This allows defining multiple Consul clients dynamically.
	gs.Group("${spring.consul.instances}", newClient, nil)
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
