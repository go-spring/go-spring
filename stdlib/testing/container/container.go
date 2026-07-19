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

// Package container is a Testcontainers-style helper for slice tests: it starts
// a real dependency (redis, mysql, ...) in a Docker container from inside a Go
// test and tears it down automatically when the test ends.
//
// Unlike a starter's check.sh (a shell smoke test run out of process), this
// helper is meant for in-process `go test`: [Run] blocks until the container is
// ready, registers cleanup with the test's tb.Cleanup, and hands back the
// dynamically mapped host:port so the test can dial the service directly.
//
// It shells out to the `docker` CLI via os/exec rather than importing a Docker
// SDK, which keeps stdlib's zero-dependency rule and reuses the repository's
// existing Docker convention (a plain docker binary, no compose-v2 plugin). Any
// proxy needed to pull an image is inherited from the environment, so export it
// before `go test` exactly as the starter check.sh scripts do.
//
// Tests that use this helper should guard with [SkipIfNoDocker] so they skip
// cleanly where Docker is unavailable (CI without a daemon, restricted laptops).
package container

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// dockerAvailable is resolved once: Docker is usable only if the binary exists
// and `docker version` (which talks to the daemon) succeeds.
func dockerAvailable() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	return exec.Command("docker", "version").Run() == nil
}

// SkipIfNoDocker skips the test when Docker is not usable, so container-backed
// integration tests do not fail on machines without a daemon.
func SkipIfNoDocker(tb TB) {
	tb.Helper()
	if !dockerAvailable() {
		tb.Skipf("docker not available; skipping container integration test")
	}
}

// TB is the subset of *testing.T this package needs. Depending on the interface
// keeps stdlib's exported API free of a testing import and lets callers pass a
// fake in their own tests.
type TB interface {
	Helper()
	Cleanup(func())
	Fatalf(format string, args ...any)
	Logf(format string, args ...any)
	Skipf(format string, args ...any)
}

// Request describes a container to start.
type Request struct {
	// Image is the Docker image reference, e.g. "redis:7". Required.
	Image string
	// Env sets container environment variables (docker run -e).
	Env map[string]string
	// Cmd overrides the image's default command/args.
	Cmd []string
	// ExposedPorts are container ports to publish to random host ports, each as
	// "port" or "port/proto" (e.g. "6379", "3306/tcp"). Run publishes with -P
	// after declaring them with --expose, then reads back the host mapping.
	ExposedPorts []string
	// WaitFor blocks Run until the container is ready. Nil means return as soon
	// as the container is started (rarely what you want).
	WaitFor Wait
}

// Container is a started container handle. Its ports are already mapped and its
// removal is already registered with the test's Cleanup.
type Container struct {
	id    string
	tb    TB
	ports map[string]string // "6379/tcp" -> host "127.0.0.1:49153"
}

// Run starts the container described by req, waits for readiness, and registers
// automatic removal with tb.Cleanup. Any docker error is fatal to the test, so a
// misconfigured request fails at Run rather than on first use.
func Run(tb TB, req Request) *Container {
	tb.Helper()
	if req.Image == "" {
		tb.Fatalf("container: Image is required")
		return nil
	}

	args := []string{"run", "-d", "-P"}
	for _, p := range req.ExposedPorts {
		args = append(args, "--expose", portOnly(p))
	}
	for k, v := range req.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, req.Image)
	args = append(args, req.Cmd...)

	out, err := run(args...)
	if err != nil {
		tb.Fatalf("container: docker run: %v", err)
		return nil
	}
	id := strings.TrimSpace(out)

	c := &Container{id: id, tb: tb, ports: map[string]string{}}
	tb.Cleanup(func() {
		if _, err := run("rm", "-f", id); err != nil {
			tb.Logf("container: docker rm -f %s: %v", short(id), err)
		}
	})

	if err := c.resolvePorts(req.ExposedPorts); err != nil {
		tb.Fatalf("container: %v", err)
		return nil
	}

	if req.WaitFor != nil {
		if err := req.WaitFor.wait(c); err != nil {
			tb.Fatalf("container %s not ready: %v\n%s", req.Image, err, c.tailLogs())
			return nil
		}
	}
	return c
}

// resolvePorts fills c.ports by asking docker for the host mapping of each
// exposed container port.
func (c *Container) resolvePorts(exposed []string) error {
	for _, p := range exposed {
		key := withProto(p)
		out, err := run("port", c.id, key)
		if err != nil {
			return fmt.Errorf("docker port %s: %w", key, err)
		}
		hostPort := firstLine(out)
		if hostPort == "" {
			return fmt.Errorf("no host mapping for %s", key)
		}
		// docker prints "0.0.0.0:49153"; normalize the wildcard host to loopback
		// so dialing works on every platform.
		c.ports[key] = normalizeHost(hostPort)
	}
	return nil
}

// Endpoint returns the dialable "host:port" for a container port ("6379" or
// "6379/tcp"). It is fatal to ask for a port that was not exposed.
func (c *Container) Endpoint(port string) string {
	c.tb.Helper()
	if ep, ok := c.ports[withProto(port)]; ok {
		return ep
	}
	c.tb.Fatalf("container: port %s was not exposed", port)
	return ""
}

// Host returns the host containers are reachable on (always loopback here).
func (c *Container) Host() string { return "127.0.0.1" }

// MappedPort returns just the host-side port number for an exposed container
// port.
func (c *Container) MappedPort(port string) string {
	c.tb.Helper()
	ep := c.Endpoint(port)
	if _, p, err := net.SplitHostPort(ep); err == nil {
		return p
	}
	return ep
}

// ID returns the container ID.
func (c *Container) ID() string { return c.id }

// tailLogs returns the container's recent logs to enrich a readiness failure
// message.
func (c *Container) tailLogs() string {
	out, _ := run("logs", "--tail", "20", c.id)
	return out
}

// run executes a docker subcommand and returns its combined stdout, or an error
// carrying stderr for diagnosis.
func run(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// --- small string helpers -------------------------------------------------

func portOnly(p string) string {
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return p
}

func withProto(p string) string {
	if strings.ContainsRune(p, '/') {
		return p
	}
	return p + "/tcp"
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func normalizeHost(hostPort string) string {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort
	}
	if host == "0.0.0.0" || host == "::" || host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// --- wait strategies -------------------------------------------------------

// Wait blocks until a started container is considered ready.
type Wait interface {
	wait(c *Container) error
}

// WaitForListeningPort waits until the host-mapped address of the given
// container port accepts a TCP connection, polling until timeout. This is the
// cheapest, most portable readiness check.
func WaitForListeningPort(port string, timeout time.Duration) Wait {
	return waitPort{port: port, timeout: timeout}
}

type waitPort struct {
	port    string
	timeout time.Duration
}

func (w waitPort) wait(c *Container) error {
	addr := c.Endpoint(w.port)
	deadline := time.Now().Add(w.timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("port %s (%s) not listening after %s", w.port, addr, w.timeout)
}

// WaitForLog waits until the container's logs contain substr, polling until
// timeout. Use it when a service accepts connections before it is truly ready
// and prints a "ready" line (e.g. databases running init scripts).
func WaitForLog(substr string, timeout time.Duration) Wait {
	return waitLog{substr: substr, timeout: timeout}
}

type waitLog struct {
	substr  string
	timeout time.Duration
}

func (w waitLog) wait(c *Container) error {
	deadline := time.Now().Add(w.timeout)
	for time.Now().Before(deadline) {
		out, err := run("logs", c.id)
		if err == nil && strings.Contains(out, w.substr) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("log line %q not seen after %s", w.substr, w.timeout)
}
