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

package container_test

import (
	"bufio"
	"net"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/container"
)

// TestRedis_Integration is the acceptance slice test: it starts a real redis
// container, talks to it, and lets the helper tear it down automatically. It
// skips cleanly where Docker is unavailable.
//
// To avoid a client dependency (stdlib is zero-dep) it speaks the Redis inline
// protocol by hand: send "PING\r\n", expect "+PONG". That is enough to prove the
// container is a genuine, reachable redis and not a stub.
func TestRedis_Integration(t *testing.T) {
	container.SkipIfNoDocker(t)

	addr := container.Redis(t) // started, port-mapped, cleanup registered

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	assert.Error(t, err).Nil()
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte("PING\r\n"))
	assert.Error(t, err).Nil()

	line, err := bufio.NewReader(conn).ReadString('\n')
	assert.Error(t, err).Nil()
	assert.String(t, line).Equal("+PONG\r\n")
}
