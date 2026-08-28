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

// client.go is the "resource entity + lifecycle" concept of this starter: the
// pulsar.Client bean's teardown (destroyClient) plus the metricsServers registry
// that tracks the per-instance /metrics server across the client's lifetime.
// pulsar exposes no wrapper entity worth holding (the bean is the raw
// pulsar.Client), so unlike the redis/memcached starters there is no Client
// wrapper type here — only the destroy path that releases the client, its
// resilience executor, and its metrics server.
package StarterPulsar

import (
	"context"
	"net/http"
	"sync"

	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/log"
)

// metricsServers tracks the /metrics HTTP server started for each client so
// destroyClient can shut it down. gs.Group's destroy callback only receives the
// bean (the pulsar.Client), so the server is keyed by the client here rather
// than being carried on the bean itself, which would change the injected type.
var metricsServers sync.Map // pulsar.Client -> *http.Server

// destroyClient closes the Pulsar client, which releases all producers and
// consumers held by it, then shuts down its /metrics server if one was started.
// When a resilience executor is attached its Close releases any background
// resources of a production driver.
func destroyClient(cl pulsar.Client) error {
	closeResilience(cl)
	cl.Close()
	shutdownMetrics(cl)
	return nil
}

// shutdownMetrics shuts down and forgets the /metrics server started for cl, if
// any. It is shared by destroyClient (normal teardown) and newClient (error
// paths after the client has been built).
func shutdownMetrics(cl pulsar.Client) {
	if v, ok := metricsServers.LoadAndDelete(cl); ok {
		if err := v.(*http.Server).Shutdown(context.Background()); err != nil {
			log.Warnf(context.Background(), log.TagAppDef, "pulsar: metrics server shutdown failed: %v", err)
		}
	}
}
