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

package observe

// ResetInstruments drops the memoized per-prefix operation sets so the next use
// resolves fresh ones. It exists for tests that install their own
// MeterProvider: the per-prefix sets are memoized, so a test running after one
// that already resolved them would otherwise keep reporting into the earlier
// provider. The always-on server set is resolved by [NewServerObserver] at
// construction and needs no reset.
func ResetInstruments() {
	resetOperationInstruments()
}

// resetOperationInstruments drops the memoized per-prefix sets, for the same
// reason [ResetInstruments] does.
func resetOperationInstruments() {
	opServerInstruments.mu.Lock()
	defer opServerInstruments.mu.Unlock()
	opServerInstruments.m = map[string]*opServerInstrumentSet{}
}
