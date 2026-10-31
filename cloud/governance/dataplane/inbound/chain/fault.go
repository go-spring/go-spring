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

package chain

import (
	"context"

	kfault "go-spring.org/cloud/governance/capability/fault"
)

// Fault is the inbound fault-injection stage as the seam sees it. The
// implementation lives in the fault package; wiring injects it, so admission
// depends on this interface (and on the kernel's decision type) rather than on
// the inbound fault stage.
type Fault interface {
	// Fire decides whether this request is set on fire, applying a configured
	// latency before returning. The returned error is the injected failure, or
	// the context error when an injected delay was cancelled; nil means the
	// request proceeds.
	Fire(ctx context.Context, req Request) (kfault.Result, error)
}
