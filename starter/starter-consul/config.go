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

import "time"

// Config defines Consul client connection configuration.
type Config struct {
	// Address is the Consul HTTP API address, e.g., "127.0.0.1:8500".
	Address string `value:"${address}"`

	// Scheme is the URI scheme for the Consul server, "http" or "https", default is "http".
	Scheme string `value:"${scheme:=http}"`

	// Datacenter is the datacenter to use, default uses the agent's datacenter.
	Datacenter string `value:"${datacenter:=}"`

	// Token is the ACL token used for requests, default is empty.
	Token string `value:"${token:=}"`

	// Namespace is the Consul Enterprise namespace, default is empty.
	Namespace string `value:"${namespace:=}"`

	// WaitTime limits how long a blocking query waits, 0 uses the agent default, e.g., "0".
	WaitTime time.Duration `value:"${wait-time:=0}"`
}
