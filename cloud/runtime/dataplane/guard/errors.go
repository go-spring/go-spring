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

package guard

import "errors"

// The refusal sentinels carried by [Result.Err]. They are this package's own,
// not chain's: an inbound refusal is not an outbound chain verdict, and an
// adapter that switches on [Result.Verdict] never needs them at all. They exist
// so a caller that wants errors.Is has a stable word.
var (
	// ErrRateLimited means the instance's rate budget was exhausted.
	ErrRateLimited = errors.New("guard: rate limited")

	// ErrBulkheadFull means the instance already handles the maximum number of
	// concurrent requests.
	ErrBulkheadFull = errors.New("guard: bulkhead full")

	// ErrCircuitOpen means the instance's circuit breaker is open.
	ErrCircuitOpen = errors.New("guard: circuit open")
)

// errNotSettled is what a deadline fallback settles with: the handler outlived
// the request's deadline without reporting, so the sample is unknown.
var errNotSettled = errors.New("guard: request not settled before its deadline")
