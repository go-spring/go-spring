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

package main

import (
	"context"

	"go-spring.org/spring/discovery"
)

// This file plays the part a company's adapter would: it implements the single
// discovery.Discovery interface against its own naming service and registers it
// once under the default backend name. The Elasticsearch instance configured
// with `service-name` (see conf/app.properties) then gets its node addresses
// from this backend at startup, with no per-component wiring.
//
// staticDiscovery is the simplest possible backend: a fixed address that never
// changes. A real adapter would talk to Consul/Nacos/an internal registry. The
// ES client resolves node addresses only once at startup, so Watch is unused.

func init() {
	discovery.Register("default", staticDiscovery{addr: "127.0.0.1:9200"})
}

type staticDiscovery struct {
	addr string
}

func (d staticDiscovery) Resolve(_ context.Context, _ string) ([]discovery.Endpoint, error) {
	return []discovery.Endpoint{{Addr: d.addr, Healthy: true}}, nil
}

func (d staticDiscovery) Watch(_ context.Context, _ string) (discovery.Watcher, error) {
	return &staticWatcher{done: make(chan struct{})}, nil
}

// staticWatcher never reports a change; Next blocks until Stop is called.
type staticWatcher struct {
	done chan struct{}
}

func (w *staticWatcher) Next() ([]discovery.Endpoint, error) {
	<-w.done
	return nil, context.Canceled
}

func (w *staticWatcher) Stop() error {
	close(w.done)
	return nil
}
