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

package StarterGrpcServer

import (
	"context"
	"errors"
	"testing"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/inbound"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/observe"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestMapServerPolicyError_FollowsDeclaredStatus pins the status-family
// translation: a 4xx answer reaches the client as InvalidArgument (the caller's
// mistake), a 5xx one as Unavailable. Passing the raw error through made
// everything cross as codes.Unknown — a code no retry policy or breaker can act
// on.
func TestMapServerPolicyError_FollowsDeclaredStatus(t *testing.T) {
	cases := []struct {
		status int
		want   codes.Code
	}{
		{400, codes.InvalidArgument},
		{429, codes.InvalidArgument},
		{503, codes.Unavailable},
		{0, codes.Unavailable}, // unvalidated direct config
	}
	for _, tc := range cases {
		inj := serverfault.NewManager()
		inj.Refresh(serverfault.ServerConfig{Rules: []serverfault.ServerRule{{
			RuleCore: kfault.RuleCore{Name: "fire", Service: "grpc", Rate: 1},
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
		in, err := inbound.Over("grpc", "grpc:test", gm, nil, inj)
		if err != nil {
			gm.Close()
			t.Fatal(err)
		}
		err = in.Run(context.Background(), observe.ServerOperation{Action: "/pkg.Svc/Method"},
			func(context.Context) error { return nil })
		gm.Close()

		var ref *inbound.Refusal
		if !errors.As(err, &ref) {
			t.Fatalf("status %d: want a refusal, got %v", tc.status, err)
		}
		if got := status.Code(mapServerPolicyError(ref)); got != tc.want {
			t.Fatalf("status %d: code %v, want %v", tc.status, got, tc.want)
		}
	}
}

// TestMapServerPolicyError_UsesDeclaredBody: a declared body becomes the gRPC
// status message, so a drill can answer with the same shape a real error has.
func TestMapServerPolicyError_UsesDeclaredBody(t *testing.T) {
	inj := serverfault.NewManager()
	inj.Refresh(serverfault.ServerConfig{Rules: []serverfault.ServerRule{{
		RuleCore: kfault.RuleCore{Name: "fire", Service: "grpc", Rate: 1},
		Status:   503,
		Body:     "downstream exploded",
	}}})
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	gm, err := admission.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	in, err := inbound.Over("grpc", "grpc:test", gm, nil, inj)
	if err != nil {
		gm.Close()
		t.Fatal(err)
	}
	err = in.Run(context.Background(), observe.ServerOperation{Action: "/pkg.Svc/Method"},
		func(context.Context) error { return nil })
	gm.Close()

	var ref *inbound.Refusal
	if !errors.As(err, &ref) {
		t.Fatalf("want a refusal, got %v", err)
	}
	st, _ := status.FromError(mapServerPolicyError(ref))
	if st.Message() != "downstream exploded" {
		t.Fatalf("message = %q, want the declared body", st.Message())
	}
}
