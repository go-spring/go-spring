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

	"go-spring.org/cloud/governance"
	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/observe"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestMapServerPolicyError_FollowsFaultClass pins the RPC half of the business
// classification: a class=business fault must reach the client as InvalidArgument
// (the caller's mistake), while a transient one is Unavailable. The old behavior
// passed the raw error through, so both crossed as codes.Unknown — a code no
// retry policy or breaker can act on.
func TestMapServerPolicyError_FollowsFaultClass(t *testing.T) {
	cases := []struct {
		class string
		want  codes.Code
	}{
		{kfault.ClassBusiness, codes.InvalidArgument},
		{kfault.ClassPermanent, codes.Unavailable},
		{"", codes.Unavailable},
	}
	for _, tc := range cases {
		inj := serverfault.NewManager()
		inj.Refresh(kfault.Config{Rules: []kfault.Rule{{
			Name: "fire", Service: "grpc", Rate: 1, Class: tc.class,
		}}})
		gm := admission.NewManager()
		in, err := governance.InboundOver("grpc", "grpc:test", gm, inj)
		if err != nil {
			gm.Close()
			t.Fatal(err)
		}
		err = in.Run(context.Background(), observe.ServerOperation{Action: "/pkg.Svc/Method"},
			func(context.Context) error { return nil })
		gm.Close()

		var ref *governance.Refusal
		if !errors.As(err, &ref) {
			t.Fatalf("class %q: want a refusal, got %v", tc.class, err)
		}
		if got := status.Code(mapServerPolicyError(ref)); got != tc.want {
			t.Fatalf("class %q: code %v, want %v", tc.class, got, tc.want)
		}
	}
}
