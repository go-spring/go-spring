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

package resilience

import (
	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/spring/gs"
)

// The manager and the bundled driver are registered here, by the package that
// owns them, so linking this package is what puts them in the container: a
// starter that injects *resilience.Manager has already imported it, and
// its constructor parameter is REQUIRED (`gs.IndexArg(n, gs.TagArg(""))`) — the
// bean being absent would fail startup, not degrade silently.
//
// This is the OUTBOUND authority only. Inbound requests run through
// go-spring.org/cloud/runtime/dataplane/inbound/admission, whose manager is owned by the governance center.
//
// The manager is self-sufficient: ExecutorFor hands out a lazily bound
// executor that reads the manager's current policy, so it works with no center
// anywhere, over the bundled driver. A governance center, when one is linked,
// only pushes policy into it.
func init() {
	// The driver is injected BY NAME here, from a static key: ${spring.resilience.driver}
	// names the [Driver] bean to build executors with, defaulting to the bundled
	// one. Each backend contributes its Driver under its own bean name (the
	// bundled driver below as "default", a backend starter's as "sentinel", ...),
	// so this one tag is where the backend is configured — no directory, no name
	// lookup at runtime.
	//
	// It is a static key, not a pushed policy: the backend is fixed for the
	// process's lifetime, because swapping it drops every executor (and so every
	// breaker and limiter state). Naming a backend no starter
	// contributed fails wiring — the tag cannot resolve the bean — so an unusable
	// selection fails startup rather than silently falling back.
	//
	// Each bean owns its own teardown: close runs on the corresponding Close,
	// which releases that direction's registered objects so a driver can stop its
	// background resources (metric pumps) rather than leak them with the process.
	// gs destroys beans in reverse construction order, so the clients and servers
	// holding those objects are gone by then.
	gs.Provide(func(d Driver) *Manager {
		return NewManager(d)
	}, gs.TagArg("${"+DriverProperty+":="+DefaultDriverName+"}")).
		Destroy((*Manager).Close).Caller(1)

	// The bundled engine, contributed as the [Driver] bean NAMED
	// [DefaultDriverName] — the driver the tag above resolves when
	// ${spring.resilience.driver} is unset or "default". It takes the process's
	// cluster limiter IF a backend starter contributed one — a Redis store, say,
	// which is what puts a rate limit beyond the process and onto every replica.
	// With no such bean the injection is nil and each executor counts in a budget
	// of its own, which still means one budget per service: the manager builds one
	// executor per label, shared by every caller of it. A process that configures
	// driver=sentinel still gets this bean; it simply is not the one selected.
	gs.Provide(func(c ratelimit.ClusterLimiter) Driver { return NewDefaultDriver(c) }, gs.TagArg("?")).
		Name(DefaultDriverName).
		Export(gs.As[Driver]()).Caller(1)
}
