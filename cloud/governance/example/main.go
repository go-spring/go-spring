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

// Command example demonstrates the governance self-built refresh chain: rules
// live in their OWN file (conf/governance.yaml) watched by cloud/governance's
// built-in file source, NOT in app.properties. Edit conf/governance.yaml while the app runs
// (e.g. change attempt-timeout, flip enabled, add a rule) and watch the
// printed policy change within a second — no restart, and no app-wide
// property re-bind.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/gs"
)

// printer prints the resolved policy AND the fault-injection config for one
// service label every second, so a rules-file edit is visible in the log
// without any client wiring — resilience and fault ride the same source.
//
// center is the governance center bean — the family's sole injection point; the
// printer reads its resilience and fault authorities off it.
type printer struct {
	center *governance.Center
}

func (p *printer) Run(ctx context.Context) error {
	// Print from a background goroutine: a Runner that blocks would hold up app
	// startup, so gs would not yet be listening for SIGTERM when the example
	// signals itself.
	go func() {
		tk := time.NewTicker(time.Second)
		defer tk.Stop()
		for i := 0; ; i++ {
			pol := p.center.Resilience().ClientPolicyFor("demo:service")
			fmt.Printf("policy: enabled=%v timeout=%v retries=%d rate-limit=%v", !pol.IsZero(), pol.AttemptTimeout, pol.MaxRetries, pol.RateLimit)
			if in := p.center.Fault(); in != nil {
				// Both directions' fires are printed: they are independent, so the
				// example shows one being switched without moving the other.
				cf, sf := in.ClientConfig(), in.ServerConfig()
				fmt.Printf(" | fault: client(enabled=%v rate=%v) server(enabled=%v rate=%v)",
					cf.Enabled, cf.Rate, sf.Enabled, sf.Rate)
			}
			fmt.Println()
			if i == 3 {
				fmt.Println(">>> edit conf/governance.yaml now: policy AND fault toggle live (e.g. set client.fault.enabled=true)")
			}
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
			}
		}
	}()
	return nil
}

func init() {
	// The governance center is taken as a bean rather than resolved from a
	// process-wide seam: cloud/governance registers it, and the "?" makes an
	// absent bean (a container that never linked it) nil rather than a wiring
	// error — the same contract every client starter's governance param uses. A
	// nil center is normalized to a fresh one over an unarmed manager so the print
	// loop never dereferences nil (an unarmed manager reads a zero policy, which
	// is exactly "governance off").
	gs.Provide(func(center *governance.Center) *printer {
		if center == nil {
			center = governance.NewCenter(governance.Config{}, resilience.NewManager(nil), nil, nil, nil, nil)
		}
		return &printer{center: center}
	},
		gs.IndexArg(0, gs.TagArg("?")), // nullable *governance.Center bean
	).Export(gs.As[gs.Runner]())
}

var manual = flag.Bool("manual", false, "run in manual verification mode (stay up until killed)")

func main() {
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	flag.Parse()
	if runtime.GOOS == "windows" {
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM) // no-op; keeps syscall import meaningful
	}

	if !*manual {
		go func() {
			// Give the operator a few seconds to try an edit, then exit so the
			// smoke script can assert on the output.
			time.Sleep(6 * time.Second)
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		}()
	}

	gs.Run()
}
