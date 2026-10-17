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

package runtime

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/runtime/dataplane/fault"
	"go-spring.org/cloud/runtime/dataplane/guard"
	"go-spring.org/cloud/runtime/dataplane/observability"
)

// TestInboundOver_InjectsFaultThroughGuard pins the single inbound seam: the
// fault authority is folded into the guard as its innermost stage, so a server
// adapter needs one Inbound and cannot double-fire.
func TestInboundOver_InjectsFaultThroughGuard(t *testing.T) {
	inj := fault.NewServerManager(nil)
	inj.Refresh(fault.Config{Rules: []fault.Rule{{Rate: 1, Error: "generic"}}})
	gm := guard.NewManager()
	defer gm.Close()

	in := InboundOver("gin", "gin:test", gm, inj)
	ran := false
	err := in.Run(context.Background(), observability.ServerOperation{Action: "GET /x"},
		func(context.Context) error {
			ran = true
			return nil
		})

	var ref *Refusal
	if !errors.As(err, &ref) {
		t.Fatalf("want an injected refusal, got %v", err)
	}
	if ref.Verdict() != guard.VerdictFaultInjected {
		t.Fatalf("want fault injected, got %v", ref.Verdict())
	}
	if ran {
		t.Fatal("an injected fault must not reach the handler")
	}
}

// TestInboundOver_NoInjectorIsGovernanceOnly pins that omitting the fault
// authority leaves a plain, admitting inbound: fault is optional, not a second
// always-on path.
func TestInboundOver_NoInjectorIsGovernanceOnly(t *testing.T) {
	gm := guard.NewManager()
	defer gm.Close()

	ran := false
	err := InboundOver("gin", "gin:test", gm).Run(context.Background(),
		observability.ServerOperation{Action: "GET /x"}, func(context.Context) error {
			ran = true
			return nil
		})
	if err != nil {
		t.Fatalf("want the request admitted, got %v", err)
	}
	if !ran {
		t.Fatal("the handler did not run")
	}
}
