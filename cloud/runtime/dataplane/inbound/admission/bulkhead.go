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

// bulkhead.go is the inbound view of the shared concurrency-isolation stage.
// The mechanism itself is direction-neutral and lives in
// [go-spring.org/cloud/runtime/dataplane/kernel/bulkhead]; this file keeps the
// admission-facing names so a caller assembling a custom middleware does not
// need to import the kernel package.

package admission

import (
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/bulkhead"
)

// Bulkhead is the kernel's concurrency-isolation stage, under the name the
// admission package exposes.
type Bulkhead = bulkhead.Bulkhead

// NewBulkhead builds a bulkhead admitting at most maxConcurrent concurrent
// requests. A request that finds the bulkhead full may wait up to queueFor for
// a slot; 0 rejects immediately.
func NewBulkhead(maxConcurrent int, queueFor time.Duration) *Bulkhead {
	return bulkhead.New(maxConcurrent, queueFor)
}
