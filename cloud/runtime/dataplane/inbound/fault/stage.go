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

	"go-spring.org/cloud/runtime/dataplane/inbound/admission"
)

// Stage adapts this package's inbound shell to guard's [admission.Fault] interface,
// so the fault authority is folded into the admission engine as its innermost
// stage. The shell is created once per server instance (not per request) and is
// fed the request's guard-level identity.
func Stage(system string, srv *Server) admission.Fault {
	return &stage{system: system, srv: srv}
}

// stage is the adapter: it translates guard's neutral request into the fault
// authority's Target and its Decision back into guard's FaultResult.
type stage struct {
	system string
	srv    *Server
}

func (s *stage) Fire(ctx context.Context, req admission.Request) (admission.FaultResult, error) {
	d, err := s.srv.Fire(ctx, Target{
		Service:  s.system,
		Action:   req.Action,
		Metadata: req.Metadata,
	})
	return admission.FaultResult{
		Rule:  d.Rule,
		Fired: d.Fired(),
		Kind:  d.Kind,
		Delay: d.Delay,
	}, err
}
