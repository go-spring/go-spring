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

package StarterPulsar

import (
	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/spring/gs"
)

func init() {

	// Register multiple Pulsar clients as a group.
	// Each instance is created according to the configuration in "${spring.pulsar.instances}".
	// This allows defining multiple Pulsar clients dynamically.
	gs.Group("${spring.pulsar.instances}", newClient, destroyClient)
}

// newClient creates a new Pulsar client based on the provided configuration.
func newClient(c Config) (pulsar.Client, error) {
	return pulsar.NewClient(pulsar.ClientOptions{
		URL:               c.URL,
		OperationTimeout:  c.OperationTimeout,
		ConnectionTimeout: c.ConnectionTimeout,
	})
}

// destroyClient closes the Pulsar client.
func destroyClient(cl pulsar.Client) error {
	cl.Close()
	return nil
}
