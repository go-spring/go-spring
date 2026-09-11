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

package StarterRegistry

// RegistrationConfig binds the instance identity under ${spring.registry}.
// These fields describe the instance itself and are shared by every registry
// backend this process registers into: configuring multiple centers
// (spring.registry.etcd.*, spring.registry.zookeeper.*, ...) fans ONE
// publication out to all of them — same service name, same address, same
// weight everywhere.
type RegistrationConfig struct {
	// ServiceName is the service this instance belongs to. Its presence is the
	// registration intent signal: set means this process publishes itself into
	// every configured registry center, unset means pure consumer.
	ServiceName string `value:"${service-name:=}"`

	// ID identifies this instance within the service; empty derives one.
	ID string `value:"${id:=}"`

	// Addr is the instance's host:port as consumers dial it.
	Addr string `value:"${addr:=}"`

	// Weight is the initial load-balancing weight; 0 means drained.
	Weight int `value:"${weight:=100}"`

	// Metadata carries backend-agnostic attributes (zone, version, ...).
	Metadata map[string]string `value:"${metadata:=}"`
}
