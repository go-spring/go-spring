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

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/inbound"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/observe"
)

// TestHTTPStatusFor_FollowsDeclaredStatus pins that an injected inbound fault
// answers with the status its rule declared. An unset status on a direct,
// unvalidated config falls back to 503 rather than writing an invalid code.
func TestHTTPStatusFor_FollowsDeclaredStatus(t *testing.T) {
	cases := []struct {
		status int
		want   int
	}{
		{http.StatusBadRequest, http.StatusBadRequest},
		{http.StatusTooManyRequests, http.StatusTooManyRequests},
		{http.StatusServiceUnavailable, http.StatusServiceUnavailable},
		{0, http.StatusServiceUnavailable}, // unvalidated direct config
	}
	for _, tc := range cases {
		inj := serverfault.NewManager()
		inj.Refresh(serverfault.ServerConfig{Rules: []serverfault.ServerRule{{
			RuleCore: kfault.RuleCore{Name: "fire", Service: "gin", Rate: 1},
			Status:   tc.status,
		}}})
		fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
		if err != nil {
			t.Fatal(err)
		}
		gm, err := admission.NewManager(fact)
		if err != nil {
			t.Fatal(err)
		}
		in, err := inbound.Over("gin", "gin:test", gm, nil, inj)
		if err != nil {
			gm.Close()
			t.Fatal(err)
		}
		err = in.Run(context.Background(), observe.ServerOperation{Action: "GET /x"},
			func(context.Context) error { return nil })
		gm.Close()

		var ref *inbound.Refusal
		if !errors.As(err, &ref) {
			t.Fatalf("status %d: want a refusal, got %v", tc.status, err)
		}
		if got := httpStatusFor(ref); got != tc.want {
			t.Fatalf("status %d: got %d, want %d", tc.status, got, tc.want)
		}
	}
}
