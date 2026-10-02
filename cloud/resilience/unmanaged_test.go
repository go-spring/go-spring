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
	"testing"
)

// TestUnmanagedNeverProtects pins that [Unmanaged] is observation only: the
// container-less degradation runs every call, whatever protection a policy
// would have applied.
func TestUnmanagedNeverProtects(t *testing.T) {
	unmanaged := Unmanaged("redis", "redis:cache")
	for i := range 50 {
		if err := unmanaged.Execute(context.Background(), func(context.Context) error { return nil }); err != nil {
			t.Fatalf("unmanaged executor rejected call %d: %v", i, err)
		}
	}
}
