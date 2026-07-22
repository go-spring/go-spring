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
	// Multi-instance only: bind a map of clients under "${spring.etcd}" and
	// register one named *clientv3.Client per entry, matching the client-starter
	// archetype (no default singleton). Each entry is an independent etcd cluster
	// connection, so a destroy hook closes it on shutdown.
	gs.Group("${spring.etcd}", newClient, destroyClient)
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
