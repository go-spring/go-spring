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
	"go-spring.org/spring/cloud/discovery"
)

// This file plays the part a company's adapter would: it registers a discovery
// backend under the default name. Here it is a fixed-address static backend (a
// real adapter would talk to Consul/Nacos/an internal registry and push fresh
// snapshots when instances come and go); the client configured with
// `service-name` (see conf/app.properties) dials the address it hands out.

func init() {
	discovery.RegisterDiscovery("default", discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:9200", Healthy: true}))
}
