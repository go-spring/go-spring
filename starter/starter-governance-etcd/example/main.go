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

// Command example demonstrates the etcd governance rule source: the rules live
// in their OWN etcd key (not in app.properties and not in a config import),
// watched by starter-governance-etcd. The example seeds a 100ms policy before
// startup, then publishes a 900ms document on the same key and observes the
// policy flip — no restart, and no app-wide property re-bind.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	rt "go-spring.org/cloud/runtime"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	clientv3 "go.etcd.io/etcd/client/v3"

	_ "go-spring.org/starter-governance-etcd"
)

const (
	etcdKey  = "/gs-govern-demo.yaml"
	etcdAddr = "127.0.0.1:2379"
)

const rulesV1 = `spring:
  governance:
    enabled: true
    client:
      default:
        enabled: true
        timeout: 100ms
        max-retries: 2
`

const rulesV2 = `spring:
  governance:
    enabled: true
    client:
      default:
        enabled: true
        timeout: 900ms
        max-retries: 2
`

// demo reads the resolved policy off the field-injected runtime, so
// a rule push is visible without any client wiring.
type demo struct {
	Center *rt.Runtime `autowire:""`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	demoBean := gs.Provide(&demo{}).Export(gs.As[gs.Rooter]())
	demo := demoBean.Interface().(*demo)

	// Seed the key BEFORE the container starts: the source's construction does
	// an initial Get, so a missing key would fail startup by design.
	if err := publish(rulesV1); err != nil {
		log.Error(log.RootFields(log.String("key", etcdKey)), log.TagAppDef, err, "seed rules failed")
		os.Exit(1)
	}

	if !*manual {
		go func() {
			// Give the source a second to finish seeding its snapshot.
			time.Sleep(time.Second)
			runTest(demo)
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("The app keeps running and prints the resolved policy every second.")
		fmt.Println("Publish a new document to", etcdKey, " (see README) and watch it flip.")
		fmt.Println("Press Ctrl+C to stop.")

		// The policy has no OnChanged hook, so polling is the observable.
		go func() {
			tk := time.NewTicker(time.Second)
			defer tk.Stop()
			for range tk.C {
				pol := demo.Center.ResilienceClient().ClientPolicyFor("demo:service")
				fmt.Printf("policy: enabled=%v timeout=%v retries=%d\n",
					!pol.IsZero(), pol.Timeout, pol.Retry.MaxRetries)
			}
		}()
	}

	gs.Run()
}

// runTest self-asserts the push chain: the cold-loaded 100ms policy, then the
// 900ms policy after publishing v2.
func runTest(d *demo) {
	ctx := log.RootFields(log.String("key", etcdKey))
	center := d.Center

	// Cold load: the seeded document is already in effect.
	pol := center.ResilienceClient().ClientPolicyFor("demo:service")
	if pol.Timeout != 100*time.Millisecond {
		err := errutil.Explain(nil, "cold-load mismatch: timeout=%v, want 100ms", pol.Timeout)
		log.Error(ctx, log.TagAppDef, err, "cold-load mismatch")
		os.Exit(1)
	}
	fmt.Println("cold-load OK: timeout is", pol.Timeout)

	// Hot push: publish v2, then poll until the center applies it.
	if err := publish(rulesV2); err != nil {
		log.Error(ctx, log.TagAppDef, err, "publish rules failed")
		os.Exit(1)
	}
	deadline := time.Now().Add(15 * time.Second)
	tk := time.NewTicker(200 * time.Millisecond)
	defer tk.Stop()
	for range tk.C {
		pol = center.ResilienceClient().ClientPolicyFor("demo:service")
		fmt.Printf("policy: enabled=%v timeout=%v retries=%d\n", !pol.IsZero(), pol.Timeout, pol.Retry.MaxRetries)
		if pol.Timeout == 900*time.Millisecond {
			fmt.Println("rule push observed: timeout is now", pol.Timeout)
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		if time.Now().After(deadline) {
			err := errutil.Explain(nil, "rule push timeout: timeout=%v", pol.Timeout)
			log.Error(ctx, log.TagAppDef, err, "rule push timeout")
			os.Exit(1)
		}
	}
}

// publish writes the rules document to the watched etcd key.
func publish(doc string) error {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdAddr},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()
	ctx, cancel := context.WithTimeout(log.RootFields(log.String("key", etcdKey)), 5*time.Second)
	defer cancel()
	_, err = cli.Put(ctx, etcdKey, doc)
	return err
}

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
