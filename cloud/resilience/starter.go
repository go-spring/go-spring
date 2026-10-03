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
	"go-spring.org/spring/gs"
)

// The manager and the bundled driver are registered here, by the package that
// owns them, so linking this package is what puts them in the container: a
// client that injects *resilience.Manager has already imported it, and its
// constructor parameter is REQUIRED (`gs.IndexArg(n, gs.TagArg(""))`) — the bean
// being absent would fail startup, not degrade silently.
//
// The manager is self-sufficient: ClientExecutorFor hands out a lazily bound
// executor that reads the manager's current policy, so it works with no center
// anywhere, over the bundled driver. A governance center, when one is linked,
// only pushes policy into it (and swaps the driver directory in).
func init() {
	// The driver directory is a constructor parameter: gs collects every [Driver]
	// bean the container holds — including this package's bundled named driver
	// below — and hands the map over at construction, so the manager never exists
	// without its directory and there is no separate install step. The tag is
	// nullable for the shape's sake rather than out of need: the bundled driver
	// is always present, so the collection is never empty.
	gs.Provide(func(drivers map[string]Driver) *Manager {
		return NewManager(drivers)
	}, gs.TagArg("?")).Caller(1)

	// The bundled engine, contributed as a NAMED driver bean. It takes the
	// process's counter store IF a backend starter contributed one — a Redis
	// store, say, which is what puts a rate limit beyond the process and onto
	// every replica. With no such bean the injection is nil and each executor
	// counts in a budget of its own, which still means one budget per
	// service: the manager builds one executor per label, shared by every caller
	// of it. A process that configures driver=sentinel still gets this bean; it
	// simply is not the one selected.
	gs.Provide(func(c Counters) Driver { return NewDefaultDriver(c) }, gs.TagArg("?")).
		Name(DefaultDriverName).
		Export(gs.As[Driver]()).Caller(1)
}
