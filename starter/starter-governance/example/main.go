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
// live in their OWN file (conf/govern.yaml) watched by starter-governance's
// file source, NOT in app.properties. Edit conf/govern.yaml while the app runs
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
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-governance"
)

// printer prints the resolved policy AND the fault-injection config for one
// resource label every second, so a rules-file edit is visible in the log
// without any client wiring — resilience and fault ride the same source.
type printer struct{}

func (p *printer) Run(ctx context.Context) error {
	// Print from a background goroutine: a Runner that blocks would hold up app
	// startup, so gs would not yet be listening for SIGTERM when the example
	// signals itself.
	go func() {
		tk := time.NewTicker(time.Second)
		defer tk.Stop()
		for i := 0; ; i++ {
			p := governance.PolicyFor("demo:resource")
			fmt.Printf("policy: enabled=%v timeout=%v retries=%d rate-limit=%v", !p.IsZero(), p.Timeout, p.MaxRetries, p.RateLimit)
			if in := fault.InjectorFor(); in != nil {
				c := in.Config()
				fmt.Printf(" | fault: enabled=%v rate=%v", c.Enabled, c.Rate)
			}
			fmt.Println()
			if i == 3 {
				fmt.Println(">>> edit conf/govern.yaml now: policy AND fault toggle live (e.g. set fault.enabled=true)")
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
	gs.Provide(&printer{}).Export(gs.As[gs.Runner]())
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
