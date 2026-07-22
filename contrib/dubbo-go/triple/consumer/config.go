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
	"time"

	"dubbo.apache.org/dubbo-go/v3/client"
)

// referenceConfig holds per-stub consumer tuning for the greet.GreetService
// reference. main.go binds it from ${spring.dubbo.references.greet} (see the
// gs.Provide that builds the stub) and turns it into dubbo-go
// client.ReferenceOption via options(), which are passed to NewGreetService so
// the autowired stub honors them on every call.
//
// It is the reference-level counterpart to starter-dubbo's client-level
// ${spring.dubbo.client.greet}: every field here overrides the client-level
// default for this one stub. All fields are optional; empty/zero keeps
// dubbo-go's own default.
//
// Enum-like fields accept the dubbo-go names:
//   - Cluster:     failover(default)|failfast|failsafe|failback|forking|available|broadcast|zoneAware
//   - LoadBalance: random(default)|roundrobin|leastactive|consistenthashing|p2c
//
// Note: retries only takes effect with cluster=failover (the default cluster).
type referenceConfig struct {
	Timeout     time.Duration `value:"${timeout:=}"`      // per-request timeout, e.g. "3s"; overrides client-level
	Retries     int           `value:"${retries:=-1}"`    // -1 keeps dubbo-go default; 0 disables; >0 retries that many times
	Cluster     string        `value:"${cluster:=}"`      // cluster strategy
	LoadBalance string        `value:"${load-balance:=}"` // load-balance strategy
}

// options translates referenceConfig into the dubbo-go ReferenceOption list
// passed to greet.NewGreetService. Empty/zero fields are skipped so dubbo-go
// keeps its own default.
func (c referenceConfig) options() []client.ReferenceOption {
	var opts []client.ReferenceOption
	if c.Timeout > 0 {
		opts = append(opts, client.WithRequestTimeout(c.Timeout))
	}
	if c.Retries >= 0 {
		opts = append(opts, client.WithRetries(c.Retries))
	}
	if c.Cluster != "" {
		opts = append(opts, client.WithCluster(c.Cluster))
	}
	if c.LoadBalance != "" {
		opts = append(opts, client.WithLoadBalance(c.LoadBalance))
	}
	return opts
}
