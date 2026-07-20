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

package StarterMesh

import (
	"testing"

	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// runSetup binds props through the starter's setup, exercising the real
// ${spring.mesh} → discovery.SetMeshMode wiring.
func runSetup(t *testing.T, props map[string]string) {
	t.Helper()
	st := flatten.NewPropertiesStorage(flatten.NewProperties(props))
	assert.Error(t, setup(nil, st)).Nil()
}

func TestSetup_EnablesMeshFromConfig(t *testing.T) {
	t.Cleanup(func() { discovery.SetMeshMode(false) })

	runSetup(t, map[string]string{"spring.mesh.enabled": "true"})
	assert.That(t, discovery.MeshMode()).True()
}

func TestSetup_DefaultDisabled(t *testing.T) {
	t.Cleanup(func() { discovery.SetMeshMode(false) })

	// A stale on-state must be turned back off when config omits the flag.
	discovery.SetMeshMode(true)
	runSetup(t, map[string]string{})
	assert.That(t, discovery.MeshMode()).False()
}

func TestSetup_InvalidValueErrors(t *testing.T) {
	t.Cleanup(func() { discovery.SetMeshMode(false) })

	st := flatten.NewPropertiesStorage(flatten.NewProperties(map[string]string{"spring.mesh.enabled": "maybe"}))
	assert.Error(t, setup(nil, st)).NotNil()
}

func TestResolveMeshMode_Explicit(t *testing.T) {
	on, err := resolveMeshMode("true")
	assert.Error(t, err).Nil()
	assert.That(t, on).True()

	off, err := resolveMeshMode("false")
	assert.Error(t, err).Nil()
	assert.That(t, off).False()
}

func TestResolveMeshMode_AutoNoSignal(t *testing.T) {
	// A clean environment carries no sidecar signal, so auto resolves to off.
	on, err := resolveMeshMode("auto")
	assert.Error(t, err).Nil()
	assert.That(t, on).False()
}

func TestResolveMeshMode_AutoWithSignal(t *testing.T) {
	t.Setenv("ISTIO_META_WORKLOAD_NAME", "user-svc")
	on, err := resolveMeshMode("auto")
	assert.Error(t, err).Nil()
	assert.That(t, on).True()
}

func TestResolveMeshMode_Invalid(t *testing.T) {
	_, err := resolveMeshMode("maybe")
	assert.Error(t, err).NotNil()
}
