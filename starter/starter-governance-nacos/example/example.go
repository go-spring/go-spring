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

// This example demonstrates the Nacos governance rule source: the rules live in
// their OWN dataId (not in app.properties and not in a config import), watched
// by starter-governance-nacos, and a publish on that dataId re-resolves the
// policy within a second — no restart, and no app-wide property re-bind.
//
//  1. The example seeds the dataId with a 100ms attempt-timeout before
//     startup, so the source's initial GetConfig succeeds.
//  2. Once running, it publishes a 900ms document to the same dataId.
//  3. The change listener delivers the new document, the source parses it and
//     pushes into the governance center; the poller observes the new timeout.
//
// The publisher client below is built directly from the HTTP open API rather
// than injected, keeping the demonstration focused on the source and its push
// chain.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-governance"
	_ "go-spring.org/starter-governance-nacos"
)

const (
	dataID    = "gs-govern-demo.yaml"
	group     = "DEFAULT_GROUP"
	nacosAddr = "127.0.0.1:8848"
)

const rulesV1 = `govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 100ms
    max-retries: 2
`

const rulesV2 = `govern:
  enabled: true
  default:
    enabled: true
    attempt-timeout: 900ms
    max-retries: 2
`

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

// poller observes the resolved policy for one resource label, so a rule push is
// visible without any client wiring.
type poller struct{}

func (p *poller) Run(ctx context.Context) error {
	// Run the whole observation in the background: a Runner that blocks would
	// hold up app startup, so gs would not yet be listening for SIGTERM when the
	// success path signals it.
	go func() {
		// Publish the updated document once the source has seeded its snapshot.
		time.Sleep(time.Second)
		if err := publish(rulesV2); err != nil {
			log.Errorf(ctx, log.TagAppDef, "publish rules failed: %v", err)
			os.Exit(1)
		}

		deadline := time.Now().Add(15 * time.Second)
		tk := time.NewTicker(200 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
			}
			pol := governance.PolicyFor("demo:resource")
			fmt.Printf("policy: enabled=%v timeout=%v retries=%d\n", !pol.IsZero(), pol.Timeout, pol.MaxRetries)
			if pol.Timeout == 900*time.Millisecond {
				fmt.Println("rule push observed: attempt-timeout is now", pol.Timeout)
				_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
				return
			}
			if time.Now().After(deadline) {
				log.Errorf(ctx, log.TagAppDef, "rule push timeout: timeout=%v", pol.Timeout)
				os.Exit(1)
			}
		}
	}()
	return nil
}

// init registers the poller as a root object so the container creates it
// eagerly.
func init() {
	gs.Provide(&poller{}).Export(gs.As[gs.Runner]())
}

func main() {
	flag.Parse()

	// Seed the dataId BEFORE the container starts: the source's construction
	// does an initial GetConfig, so a missing document would fail startup by
	// design.
	if err := publish(rulesV1); err != nil {
		log.Errorf(context.Background(), log.TagAppDef, "seed rules failed: %v", err)
		os.Exit(1)
	}

	if *manual {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Publish a new document to", dataID, "to see the policy change.")
		fmt.Println("Press Ctrl+C to stop.")
	}

	gs.Run()
}

// publish writes the rules document to the watched dataId via the Nacos HTTP
// open API (POST /nacos/v1/cs/configs).
func publish(doc string) error {
	form := url.Values{
		"dataId":  {dataID},
		"group":   {group},
		"content": {doc},
	}
	resp, err := http.PostForm("http://"+nacosAddr+"/nacos/v1/cs/configs", form)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "true" {
		return fmt.Errorf("unexpected response: status=%d body=%q", resp.StatusCode, string(body))
	}
	return nil
}

// init sets the working directory of the application to the directory where
// this source file resides, so relative file operations are based on the source
// file location rather than the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
