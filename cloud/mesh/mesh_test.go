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

package mesh

import "testing"

// These tests mutate process environment via t.Setenv, so none may call
// t.Parallel.

func TestEnabled_On(t *testing.T) {
	t.Setenv("GS_MESH", "on")
	// Forced on even with no sidecar signal.
	if !Enabled() {
		t.Fatal(`GS_MESH=on should force Enabled true`)
	}
}

func TestEnabled_Off(t *testing.T) {
	t.Setenv("ISTIO_META_WORKLOAD_NAME", "user-svc") // sidecar present
	t.Setenv("GS_MESH", "off")
	// Forced off overrides sidecar detection.
	if Enabled() {
		t.Fatal(`GS_MESH=off should force Enabled false even with a sidecar`)
	}
}

func TestEnabled_AutoNoSidecar(t *testing.T) {
	t.Setenv("GS_MESH", "auto")
	// auto + no sidecar → off, so client-side discovery stays active.
	if Enabled() {
		t.Fatal(`GS_MESH=auto with no sidecar should be false`)
	}
}

func TestEnabled_AutoWithSidecar(t *testing.T) {
	t.Setenv("GS_MESH", "auto")
	t.Setenv("ISTIO_META_WORKLOAD_NAME", "user-svc")
	// auto + sidecar detected → on, zero code config.
	if !Enabled() {
		t.Fatal(`GS_MESH=auto with a sidecar should be true`)
	}
}

func TestEnabled_EmptyIsAuto(t *testing.T) {
	t.Setenv("GS_MESH", "") // empty == unset for os.Getenv; both mean auto
	t.Setenv("ISTIO_META_WORKLOAD_NAME", "user-svc")
	if !Enabled() {
		t.Fatal(`empty/unset GS_MESH should behave as auto (sidecar detected → true)`)
	}
}

func TestEnabled_TrimLower(t *testing.T) {
	t.Setenv("GS_MESH", "  ON  ")
	if !Enabled() {
		t.Fatal(`GS_MESH should be matched case-insensitively after trimming`)
	}
}

func TestDetect_NoSignal(t *testing.T) {
	if Detect() {
		t.Fatal("Detect should be false without sidecar env vars")
	}
}

func TestDetect_IstioSignal(t *testing.T) {
	t.Setenv("ISTIO_META_WORKLOAD_NAME", "user-svc")
	if !Detect() {
		t.Fatal("Detect should be true with an ISTIO_META_* env var")
	}
}

func TestDetect_LinkerdSignal(t *testing.T) {
	t.Setenv("LINKERD2_PROXY_LOG", "info")
	if !Detect() {
		t.Fatal("Detect should be true with a LINKERD2_PROXY_* env var")
	}
}
