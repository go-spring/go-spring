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

package fault

import (
	"context"
)

// ApplyServer gates fn with the SERVER side's fault rules. It is the server-side fault
// seam — a gin middleware or gRPC interceptor wraps its handler call with
// ApplyServer to inject faults into inbound traffic, mirroring how [WrapClientExecutor]
// gates outbound/client calls. Both directions share the same [Injector] (so
// one governance config drives either), the same injection sequence
// ([Injector.gate]) and the same guardrail model — but each reads its own
// [Config], so burning inbound requests leaves outbound calls untouched.
//
// in must be non-nil: the injector bean is always present (its constructor is
// REQUIRED — an absent bean fails startup), and with no rule armed it is a
// transparent pass-through anyway. ApplyServer does NOT retry — retry is a
// client concern; see [WrapClientExecutor].
func ApplyServer(ctx context.Context, in *Injector, service string, fn func() error) error {
	return in.gate(ctx, &in.server, service, func(context.Context) error { return fn() })
}
