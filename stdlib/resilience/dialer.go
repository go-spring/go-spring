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

package resilience

import (
	"context"
	"net"
)

// DialFunc is the shape of the connection-establishing hook exposed by common
// clients — it matches net.Dialer.DialContext as well as
// [go-spring.org/stdlib/discovery.LiveDialer.DialContext], which resolves a live
// service endpoint before each dial.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// NewDialer wraps base so every connection attempt flows through exec. It is the
// client-side dialer seam of the framework: pairing it with a discovery
// LiveDialer gives service-to-service calls circuit breaking, retry and a
// bulkhead at the point connections are made, without the client library
// knowing anything about resilience.
//
// Because a dialer is already scoped to one service, resource is a fixed label
// (typically the service name) shared by every dial through it. When exec is nil
// base is returned unchanged, so wiring stays a no-op until a policy is
// configured — the same zero-config opt-in contract as the other seams.
//
// The coverage is coarser than the HTTP/RPC seams: protection keys on the dial,
// so the breaker trips on connection failures (refused, timed out) rather than
// on per-request errors of an already-open connection. That is exactly the level
// at which the widely reusable discovery LiveDialer operates.
func NewDialer(base DialFunc, exec Executor, resource string) DialFunc {
	if exec == nil {
		return base
	}
	if base == nil {
		base = (&net.Dialer{}).DialContext
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		var conn net.Conn
		err := exec.Execute(ctx, resource, func(ctx context.Context) error {
			c, err := base(ctx, network, addr)
			if err != nil {
				return err
			}
			conn = c
			return nil
		})
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
}
