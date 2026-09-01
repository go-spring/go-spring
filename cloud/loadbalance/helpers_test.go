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

package loadbalance

import (
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// eps builds a plain endpoint slice from addresses.
func eps(addrs ...string) []discovery.Endpoint {
	out := make([]discovery.Endpoint, len(addrs))
	for i, a := range addrs {
		out[i] = discovery.Endpoint{Addr: a}
	}
	return out
}

// addrs extracts the addresses from an endpoint slice (Endpoint is not
// comparable — it holds a map — so slice assertions run over addresses).
func addrs(eps []discovery.Endpoint) []string {
	out := make([]string, len(eps))
	for i, ep := range eps {
		out[i] = ep.Addr
	}
	return out
}

// counts tallies how many picks landed on each address over n calls.
func counts(t *testing.T, b Balancer, set []discovery.Endpoint, info PickInfo, n int) map[string]int {
	t.Helper()
	m := map[string]int{}
	for range n {
		ep, err := b.Pick(set, info)
		assert.Error(t, err).Nil()
		m[ep.Addr]++
		b.Complete(ep, nil)
	}
	return m
}

// staticSource is a fixed EndpointSource for pool tests.
type staticSource []discovery.Endpoint

func (s staticSource) Endpoints() []discovery.Endpoint { return s }
