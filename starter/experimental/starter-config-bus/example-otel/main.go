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

// Package main is the observability example for starter-config-bus.
//
// It broadcasts one refresh over a NATS-backed bus and then proves all three
// observability signals arrived:
//
//	trace   — a producer span linked to the subscriber's consumer span, both
//	          visible in Jaeger (docker-compose).
//	metrics — the config.bus.* instruments scraped from the Prometheus endpoint.
//	health  — the bus reports its own subscription liveness.
//
// Run with -manual to keep the server up for interactive exploration in the
// Jaeger UI and on the metrics endpoint.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-otel"

	StarterConfigBus "go-spring.org/starter-config-bus"
	_ "go-spring.org/starter-nats"
)

// Demo binds a dynamic field and holds the bus so it can broadcast a refresh.
// It is registered as a root object so the container creates it eagerly.
type Demo struct {
	Bus     *StarterConfigBus.ConfigBus `autowire:"configBus"`
	Message gs.Dync[string]             `value:"${demo.message:=none}"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	demoBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(700 * time.Millisecond)
			runTest(demoBean.Interface().(*Demo))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Printf("Jaeger UI:  http://127.0.0.1:16686 (service %s)\n", serviceName)
		fmt.Println("Metrics:    http://127.0.0.1:9090/metrics (grep config_bus)")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// serviceName matches spring.observability.service-name in conf/app.properties.
const serviceName = "config-bus-otel-example"

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}

func runTest(d *Demo) {
	// Feature 0: health — the bus reports whether its subscription is still
	// active, which is the signal a NATS connectivity check cannot give.
	if !d.Bus.Healthy() {
		fail("config bus reports unhealthy: subscription is not active")
	}

	// Feature 1: broadcast a refresh. The value is raised on an environment
	// source the running app has not read yet, so only the refresh can make it
	// visible — a publish that silently did nothing would fail the poll below.
	want := "v-" + time.Now().Format("150405")
	_ = os.Setenv("GS_DEMO_MESSAGE", want)

	// The ctx is where the producer span hangs off, so a real app calling this
	// from an HTTP handler gets the refresh linked into the request's trace.
	ctx := context.Background()
	if err := d.Bus.Publish(ctx, ""); err != nil {
		fail("publish refresh failed: %v", err)
	}
	fmt.Println("published refresh event on the bus")

	// Feature 2: the subscriber (this process, standing in for any instance in
	// the fleet) re-runs the property refresh, flipping the bound Dync field.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if got := d.Message.Value(); got == want {
			fmt.Println("bus refresh observed:", got)
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if got := d.Message.Value(); got != want {
		fail("refresh timeout: message=%q want=%q", got, want)
	}

	// Feature 3: the bus metrics must be exported, not merely recorded. Scrape
	// the Prometheus endpoint and require every instrument this example drives.
	time.Sleep(2 * time.Second)
	metrics, err := httpGet("http://127.0.0.1:9090/metrics")
	if err != nil {
		fail("scrape metrics failed: %v", err)
	}
	for _, name := range []string{
		"config_bus_events_total",        // received broadcasts, by outcome
		"config_bus_publishes_total",     // published broadcasts, by outcome
		"config_bus_refresh_duration_",   // refresh latency histogram
		"messaging_client_operation_dur", // transport instrumentation (starter-nats)
	} {
		if !strings.Contains(metrics, name) {
			fail("metric %q not found on the Prometheus endpoint", name)
		}
	}
	if !strings.Contains(metrics, `outcome="refreshed"`) {
		fail("no refreshed outcome recorded — the refresh path did not report")
	}
	fmt.Println("OK: config.bus.* and messaging.client.* metrics exported")

	// Wait for the collector to flush before asking Jaeger.
	time.Sleep(3 * time.Second)

	// Feature 4: the producer and consumer spans must form one trace. A single
	// broadcast yields both spans, so the trace for this service holds at least
	// two — that is the cross-instance link the bus exists to make visible.
	traces, err := httpGet("http://127.0.0.1:16686/api/traces?service=" + serviceName + "&limit=1")
	if err != nil {
		fail("Jaeger API request failed: %v", err)
	}
	if !strings.Contains(traces, `"data":[`) {
		fail("no traces found in Jaeger for service %q", serviceName)
	}
	for _, op := range []string{`"publish"`, `"consume"`} {
		if !strings.Contains(traces, op) {
			fail("trace is missing the %s span — the bus link is broken", op)
		}
	}
	fmt.Println("OK: publish -> consume trace found in Jaeger for", serviceName)

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
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
