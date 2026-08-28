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

package StarterAsynq

import (
	"time"

	observe "go-spring.org/cloud/observe"
	"go-spring.org/cloud/tlsconf"
)

// Config defines one Asynq instance (a Redis-backed task queue). One instance
// yields both a producer Client (enqueue) and — when server.enabled — a
// worker Server (dequeue + run), sharing the same Redis connection settings.
type Config struct {
	// Addr is the Redis server address, "host:port". Required.
	Addr string `value:"${addr}" expr:"$ != ''"`

	// Username / Password authenticate against Redis ACL. Both optional.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// DB is the Redis database index (default 0).
	DB int `value:"${db:=0}"`

	// TLS configures client-to-Redis TLS (tlsconf shared block).
	TLS tlsconf.TLSConfig `value:"${tls}"`

	// Concurrency is the maximum number of tasks the server processes
	// concurrently (default 10).
	Concurrency int `value:"${concurrency:=10}"`

	// Queues maps queue names to their priority weight; higher weight =
	// processed more often. Empty means the "default" queue at weight 1.
	Queues map[string]int `value:"${queues:=}"`

	// ShutdownTimeout is how long the server waits for in-flight tasks to
	// finish on shutdown before giving up.
	ShutdownTimeout time.Duration `value:"${shutdown-timeout:=8s}"`

	// Server controls whether the worker role is started for this instance.
	// It is off by default: a long-running worker is an opt-in, and most
	// processes only enqueue. See starter/DESIGN.md — starters do not
	// activate services the operator did not ask for.
	Server ServerConfig `value:"${server}"`

	// Observability configures the per-task access log (off/brief/detailed).
	// Instance keys (spring.asynq.<name>.observability.*) override the
	// top-level observability.* keys — see Client.resolveObservability.
	Observability observe.ObserveConfig `value:"${observability:=}"`

	// Driver specifies which Asynq driver to use; it must match a name
	// registered via RegisterDriver. Defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}

// ServerConfig controls the worker role.
type ServerConfig struct {
	// Enabled turns on the worker server (dequeue + run) for this instance.
	// Default false.
	Enabled bool `value:"${enabled:=false}"`
}
