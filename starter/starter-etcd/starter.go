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

package StarterEtcd

import (
	"go-spring.org/spring/gs"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {

	// Register a single default etcd client.
	// This client will only be created if the property "spring.etcd.endpoints" is set.
	// It uses the configuration tagged with "${spring.etcd}" and is named "__default__".
	gs.Provide(newClient, gs.TagArg("${spring.etcd}")).
		Condition(gs.OnProperty("spring.etcd.endpoints")).
		Destroy(destroyClient).
		Name("__default__")

	// Register multiple etcd clients as a group.
	// Each instance is created according to the configuration in "${spring.etcd.instances}".
	// This allows defining multiple etcd clients dynamically.
	gs.Group("${spring.etcd.instances}", newClient, destroyClient)
}

// newClient creates a new etcd client based on the provided configuration.
func newClient(c Config) (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints:            c.EndpointList(),
		Username:             c.Username,
		Password:             c.Password,
		DialTimeout:          c.DialTimeout,
		AutoSyncInterval:     c.AutoSyncInterval,
		DialKeepAliveTime:    c.DialKeepAliveTime,
		DialKeepAliveTimeout: c.DialKeepAliveTimeout,
	})
}

// destroyClient closes the etcd client.
func destroyClient(client *clientv3.Client) error {
	return client.Close()
}
