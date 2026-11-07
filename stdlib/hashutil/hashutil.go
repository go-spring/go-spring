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

// Package hashutil provides hashing helpers. [FNV1a64] computes the 64-bit
// FNV-1a hash of a string via the standard library hash/fnv, favouring
// readability and stdlib consistency over raw performance.
package hashutil

import (
	"hash/fnv"
)

// FNV1a32 computes the 32-bit FNV-1a hash of the given string.
//
// It is written as a direct loop rather than through hash/fnv: callers on hot
// paths (a consistent-hash ring, a per-request fingerprint) hash many short
// strings per call, and fnv.New32a allocates a hasher object plus a []byte
// conversion for every one of them. The result is identical.
func FNV1a32(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// FNV1a32Bytes is [FNV1a32] over a byte slice, for callers that build the input
// in a reusable buffer instead of allocating a string.
func FNV1a32Bytes(b []byte) uint32 {
	h := uint32(2166136261)
	for _, c := range b {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}

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
