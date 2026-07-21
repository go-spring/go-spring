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

// Command example demonstrates wiring starter-lock-k8s into a Go-Spring
// application to run leader election over a coordination.k8s.io/Lease, with no
// external middleware.
//
// The starter exports a lock.Locker under "${spring.lock.<name>}"; business code
// injects that interface and builds a lock.Election on top, exactly as it would
// with the etcd/consul/redis backends — switching backend is a blank-import swap.
//
// Leader election needs a real cluster, which cannot be provided locally, so the
// example is cluster-aware: when a kubeconfig (KUBECONFIG env) or in-cluster
// ServiceAccount is available it wires the Lease Locker and campaigns for
// leadership; otherwise it prints guidance and exits cleanly, so `go run .`
// never crashes outside a cluster. See deploy/ for an in-cluster run.
package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"k8s.io/client-go/rest"

	"go-spring.org/log"
	"go-spring.org/spring/cloud/lock"
	"go-spring.org/spring/gs"

	// Blank-import registers the Lease-backed Locker beans declared under
	// spring.lock.
	_ "go-spring.org/starter-lock-k8s"
)

// lockName matches the map key set under spring.lock.<lockName> below; leaderKey
// is the Lease name candidates contend for.
const (
	lockName  = "default"
	leaderKey = "example-leader"
)

// ElectionDemo injects the Lease-backed Locker and runs a leader election on it.
// It is registered as a root object only when a cluster is reachable (see main).
type ElectionDemo struct {
	Locker lock.Locker `autowire:""`
}

func main() {
	// Unset shell-leaked env vars so runs are reproducible across examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	kubeconfig := os.Getenv("KUBECONFIG")
	_, inClusterErr := rest.InClusterConfig()
	clusterAvailable := kubeconfig != "" || inClusterErr == nil

	if !clusterAvailable {
		log.Infof(context.Background(), log.TagAppDef,
			"no cluster reachable (no KUBECONFIG, not in-cluster); showing wiring only. See deploy/ to run leader election in a cluster.")
		go selfTerminateAfter(500 * time.Millisecond)
		gs.Run()
		return
	}

	demo := &ElectionDemo{}
	// Drive the election from a goroutine once bean injection has completed,
	// mirroring sibling starter examples; the container populates demo.Locker
	// during startup.
	go func() {
		time.Sleep(800 * time.Millisecond)
		demo.runElection()
	}()
	gs.Configure(func(app gs.App) {
		// Declare one Lease-backed Locker named "default". Namespace defaults to
		// "default"; kubeconfig is only set when running out-of-cluster.
		app.Property("spring.lock."+lockName+".namespace", "default")
		if kubeconfig != "" {
			app.Property("spring.lock."+lockName+".kubeconfig", kubeconfig)
		}
		// The main server is unused; keep the smoke test focused on the lock.
		app.Property("spring.http.server.enabled", "false")
		app.Provide(demo).Export(gs.As[gs.Rooter]())
	}).Run()
}

// runElection campaigns for leadership, logs the outcome, and self-terminates
// once this instance becomes leader (or after a timeout), so the example is a
// one-shot.
func (d *ElectionDemo) runElection() {
	ctx := context.Background()
	if d.Locker == nil {
		log.Errorf(ctx, log.TagAppDef, "locker was not injected")
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		return
	}

	elected := make(chan struct{}, 1)
	e := lock.NewElection(lock.ElectionConfig{
		Locker:        d.Locker,
		Key:           leaderKey,
		OnElected:     func(context.Context) { elected <- struct{}{} },
		RetryInterval: 500 * time.Millisecond,
	})

	runCtx, cancel := context.WithCancel(context.Background())
	go func() { _ = e.Run(runCtx) }()
	defer cancel()

	select {
	case <-elected:
		log.Infof(ctx, log.TagAppDef, "became leader over Lease %q (isLeader=%v)", leaderKey, e.IsLeader())
	case <-time.After(10 * time.Second):
		log.Warnf(ctx, log.TagAppDef, "did not win leadership within timeout")
	}
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func selfTerminateAfter(d time.Duration) {
	time.Sleep(d)
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// init sets the working directory to this source file's directory so
// conf/app.properties loads regardless of where the process is launched.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
}
