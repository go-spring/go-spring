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
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/spring/gs"
)

// This example wires the rate-limit root's file source. The root is isolated, so
// its keys are unprefixed and its file is its own.
func init() {
	gs.Provide(func() (*source.FileSource[resilience.RateLimitSettings], error) {
		return source.NewFileSource[resilience.RateLimitSettings]("conf/ratelimit.properties")
	}).
		Name("example.ratelimit.source").
		Destroy((*source.FileSource[resilience.RateLimitSettings]).Close).
		Export(gs.As[source.Source[resilience.RateLimitSettings]](), gs.As[gs.Runner]()).
		Caller(1)
}
