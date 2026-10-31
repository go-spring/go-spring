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

package fault

import (
	"context"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/chain"
)

// Stage adapts this package's inbound entry point to the seam's [chain.Fault]
// interface, so the fault authority is folded into the admission engine as its
// innermost stage. The entry point is created once per server instance (not per
// request) and is fed the protocol family the instance belongs to.
//
// system becomes [kfault.Target.Service], which is the granularity inbound
// fault rules match on: the FAMILY ("gin", "grpc"), not the instance label a
// governance AdmissionService uses. An inbound experiment burns every server of that
// family, and a rule written with an instance label ("gin:0.0.0.0:8080")
// matches nothing — see the documented example
// `spring.fault.server.rules[0].service=grpc`.
func Stage(system string, srv *Server) chain.Fault {
	return &stage{system: system, srv: srv}
}

// stage is the adapter: it translates the seam's request into the fault
// authority's [kfault.Target] and returns its [kfault.Result] unchanged.
type stage struct {
	system string
	srv    *Server
}

func (s *stage) Fire(ctx context.Context, req chain.Request) (kfault.Result, error) {
	return s.srv.Fire(ctx, kfault.Target{
		Service:  s.system,
		Action:   req.Action,
		Metadata: req.Metadata,
	})
}
