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

func TestEnabled_Toggle(t *testing.T) {
	t.Cleanup(func() { SetEnabled(false) })

	if Enabled() {
		t.Fatal("Enabled should start false")
	}
	SetEnabled(true)
	if !Enabled() {
		t.Fatal("Enabled should be true after SetEnabled(true)")
	}
	SetEnabled(false)
	if Enabled() {
		t.Fatal("Enabled should be false after SetEnabled(false)")
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
