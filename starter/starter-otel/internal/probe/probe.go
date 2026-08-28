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

// Package probe provides the one-shot OTLP endpoint connectivity probe. The
// OTLP exporters connect lazily and retry forever in the background, so a
// mistyped endpoint is otherwise completely silent - in contrast, the
// pull-based Prometheus exporter fails loudly when its port cannot bind. The
// probe restores that symmetry with the least noise possible: a single WARN
// per process naming the endpoint, never a startup failure (the collector
// may legitimately come up after the app).
package probe

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"go-spring.org/log"
)

// WarnOnce dials endpoint (host:port) once with the given timeout and, on
// failure, emits exactly one WARN naming the signal kind and endpoint. The
// once-guard is per (kind, endpoint) pair, so a repeated setup (e.g. across
// gs.RunTest re-runs) does not re-warn. A successful dial says nothing beyond
// "something is listening", but it catches the common misconfigurations
// (wrong host/port, collector not running, typo) that would otherwise surface
// only as silently dropped telemetry.
func WarnOnce(kind, endpoint string, timeout time.Duration) {
	if endpoint == "" {
		return
	}
	key := kind + "\x00" + endpoint
	warnMu.Lock()
	if warned[key] {
		warnMu.Unlock()
		return
	}
	warned[key] = true
	warnMu.Unlock()

	addr := normalize(endpoint)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		_ = conn.Close()
		return
	}
	log.Warnf(context.Background(), log.TagAppDef,
		"%s: OTLP endpoint %q is not reachable (probe: %v) - telemetry will be silently dropped until it comes up; check spring.observability.%s.endpoint",
		kind, endpoint, err, kind)
}

var (
	warnMu sync.Mutex
	warned = map[string]bool{}
)

// normalize strips an accidental scheme or path from the configured endpoint
// so the TCP dial gets a bare host:port.
func normalize(endpoint string) string {
	addr := endpoint
	if i := strings.Index(addr, "://"); i >= 0 {
		addr = addr[i+3:]
	}
	if i := strings.IndexAny(addr, "/"); i >= 0 {
		addr = addr[:i]
	}
	return addr
}
