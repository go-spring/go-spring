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

package StarterPProf

import (
	"go-spring.org/spring/gs"
)

func init() {
	// enablePProfServer mirrors the built-in HTTP server's enablement
	// (see spring.http.server.enabled in spring/gs/http.go): the pprof server
	// is on by default and opted out of explicitly with
	// spring.pprof.enabled=false.
	enablePProfServer := gs.OnProperty("spring.pprof.enabled").
		HavingValue("true").MatchIfMissing()

	// Registers a SimplePProfServer bean in the IoC container. It exports
	// gs.Server so the application collects it alongside the main HTTP server
	// (both gathered into the []Server collection by type).
	gs.Provide(
		NewSimplePProfServer,
		gs.IndexArg(1, gs.TagArg("${spring.pprof}")),
	).Condition(enablePProfServer).Export(gs.As[gs.Server]())
}
