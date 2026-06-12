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

package hashutil

import (
	"hash/fnv"
)

// FNV1a64 computes the 64-bit FNV-1a hash of the given string using
// the standard library hash/fnv implementation.
//
// This implementation favors readability and adherence to standard
// library abstractions over raw performance. It is appropriate for:
//   - Non-critical code paths
//   - One-off or infrequent hash computations
//   - Situations where consistency with other hash.Hash users matters
func FNV1a64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
