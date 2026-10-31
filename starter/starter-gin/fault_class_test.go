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

package StarterGin

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"go-spring.org/cloud/governance"
	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/observe"
)

// TestHTTPStatusFor_FollowsFaultClass pins the inbound half of the business
// classification: a rule that declares class=business must cross the wire as a
// caller error (400), not as "the downstream is down" (503), which is what every
// injected fault used to become.
func TestHTTPStatusFor_FollowsFaultClass(t *testing.T) {
	cases := []struct {
		class string
		want  int
	}{
		{kfault.ClassBusiness, http.StatusBadRequest},
		{kfault.ClassPermanent, http.StatusServiceUnavailable},
		{"", http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		inj := serverfault.NewManager()
		inj.Refresh(kfault.Config{Rules: []kfault.Rule{{
			Name: "fire", Service: "gin", Rate: 1, Class: tc.class,
		}}})
		gm := admission.NewManager()
		in, err := governance.InboundOver("gin", "gin:test", gm, inj)
		if err != nil {
			gm.Close()
			t.Fatal(err)
		}
		err = in.Run(context.Background(), observe.ServerOperation{Action: "GET /x"},
			func(context.Context) error { return nil })
		gm.Close()

		var ref *governance.Refusal
		if !errors.As(err, &ref) {
			t.Fatalf("class %q: want a refusal, got %v", tc.class, err)
		}
		if got := httpStatusFor(ref); got != tc.want {
			t.Fatalf("class %q: status %d, want %d", tc.class, got, tc.want)
		}
	}
}
