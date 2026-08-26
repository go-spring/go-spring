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

package StarterActuator

import (
	"net/http"
)

// BeanDescriptor is one entry of the /beans listing: the bean's name and its
// type's fully-qualified name.
type BeanDescriptor struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// BeanLister supplies the container's bean list for the /beans endpoint.
//
// Information boundary (deliberate, not a TODO): the gs core does not export
// bean enumeration. The global bean registry (gs/internal/gs_init.Beans) lives
// in an internal package that this module cannot import, and it is cleared
// after wiring anyway (gs_init.Clear at the end of a non-test Refresh), so
// even an internal caller would see an empty list at runtime. The container's
// name/type indexes are likewise internal. Extending the core just for this
// read-only endpoint was ruled out, so /beans is a contribution seam: any
// module (or the application itself) that CAN observe its bean set — e.g. a
// test harness, or a bootstrap wrapper that records what it registers —
// exports a bean as BeanLister and the actuator surfaces it here. With no
// contributor, /beans reports the boundary instead of an empty list, because
// an empty list would falsely suggest "the app has no beans".
type BeanLister interface {
	// Beans returns the bean descriptors to surface, in any order.
	Beans() []BeanDescriptor
}

// handleBeans serves GET /beans: the container's bean list as
// [{"name":..., "type":...}], the Go analogue of Spring Boot's /actuator/beans.
func (s *Server) handleBeans(w http.ResponseWriter, r *http.Request) {
	if s.BeanRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"beans": []BeanDescriptor{},
			"note": "bean registry not available: the gs core does not export bean " +
				"enumeration; provide a BeanLister bean to enable this endpoint",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"beans": s.BeanRegistry.Beans(),
	})
}
