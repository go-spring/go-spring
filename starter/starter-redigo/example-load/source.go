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

// This example wires one file source per control-plane root: the roots are
// isolated, so each reads its own file and its keys carry no capability prefix.
func init() {
	gs.Provide(func() (*source.FileSource[resilience.LimitsSettings], error) {
		return source.NewFileSource[resilience.LimitsSettings]("conf/timeoutretry.properties")
	}).
		Name("example.timeoutretry.source").
		Destroy((*source.FileSource[resilience.LimitsSettings]).Close).
		Export(gs.As[source.Source[resilience.LimitsSettings]](), gs.As[gs.Runner]()).
		Caller(1)

	gs.Provide(func() (*source.FileSource[resilience.BreakerSettings], error) {
		return source.NewFileSource[resilience.BreakerSettings]("conf/breaker.properties")
	}).
		Name("example.circuitbreaker.source").
		Destroy((*source.FileSource[resilience.BreakerSettings]).Close).
		Export(gs.As[source.Source[resilience.BreakerSettings]](), gs.As[gs.Runner]()).
		Caller(1)
}
