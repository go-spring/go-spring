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

package StarterElasticsearch

import (
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Elasticsearch clients as a group.
	// Each instance is created according to the configuration in "${spring.elasticsearch}".
	// This allows defining multiple elasticsearch instances dynamically.
	gs.Group("${spring.elasticsearch}", newClient, destroyClient)
}

// newClient creates a new Elasticsearch client based on the provided
// configuration. The cluster is probed once at startup so that misconfiguration
// or an unreachable cluster fails fast rather than on first use.
func newClient(c Config) (*elasticsearch.Client, error) {
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "elasticsearch driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create elasticsearch client")
	}
	if err := HealthCheck(client); err != nil {
		return nil, errutil.Explain(err, "failed to reach elasticsearch cluster")
	}
	return client, nil
}

// HealthCheck reports whether the Elasticsearch cluster is reachable by issuing
// an Info request. It is a thin readiness probe suitable for wiring into a
// health endpoint.
func HealthCheck(client *elasticsearch.Client) error {
	res, err := client.Info()
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return fmt.Errorf("elasticsearch: info returned %s", res.Status())
	}
	return nil
}

// destroyClient releases the Elasticsearch client. The v8 client holds no
// closable resources (its transport uses net/http with idle-connection reuse),
// so there is nothing to close; this callback exists only to keep the
// group's lifecycle handling uniform with the other clients.
func destroyClient(client *elasticsearch.Client) error {
	return nil
}
