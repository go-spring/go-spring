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

package podinfo

import (
	"os"
	"path/filepath"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestLabels_EmptyPath(t *testing.T) {
	p := &PodInfo{}
	got, err := p.Labels()
	assert.Error(t, err).Nil()
	assert.That(t, len(got)).Equal(0)
}

func TestLabels_ParsesDownwardAPIFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "labels")
	content := `app="myapp"
version="1.0.0"
tier="backend"
`
	err := os.WriteFile(path, []byte(content), 0o644)
	assert.Error(t, err).Nil()

	p := &PodInfo{LabelsPath: path}
	got, err := p.Labels()
	assert.Error(t, err).Nil()
	assert.That(t, got["app"]).Equal("myapp")
	assert.That(t, got["version"]).Equal("1.0.0")
	assert.That(t, got["tier"]).Equal("backend")
}

func TestLabels_SkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "labels")
	content := "\napp=\"myapp\"\n\n"
	err := os.WriteFile(path, []byte(content), 0o644)
	assert.Error(t, err).Nil()

	p := &PodInfo{LabelsPath: path}
	got, err := p.Labels()
	assert.Error(t, err).Nil()
	assert.That(t, len(got)).Equal(1)
	assert.That(t, got["app"]).Equal("myapp")
}

func TestLabels_MissingFile(t *testing.T) {
	p := &PodInfo{LabelsPath: filepath.Join(t.TempDir(), "does-not-exist")}
	_, err := p.Labels()
	assert.Error(t, err).Matches("no such file|cannot find")
}

func TestMetadata_OmitsEmptyFields(t *testing.T) {
	p := &PodInfo{
		Name:      "pod-abc",
		Namespace: "default",
		IP:        "10.0.0.5",
	}
	got := p.Metadata()
	assert.That(t, got["pod.name"]).Equal("pod-abc")
	assert.That(t, got["pod.namespace"]).Equal("default")
	assert.That(t, got["pod.ip"]).Equal("10.0.0.5")
	_, ok := got["node.name"]
	assert.That(t, ok).False()
	_, ok = got["pod.serviceAccount"]
	assert.That(t, ok).False()
}

func TestMetadata_AllFields(t *testing.T) {
	p := &PodInfo{
		Name:           "pod-abc",
		Namespace:      "default",
		IP:             "10.0.0.5",
		NodeName:       "node-1",
		ServiceAccount: "svc-acct",
		LabelsPath:     "/etc/podinfo/labels",
	}
	got := p.Metadata()
	assert.That(t, len(got)).Equal(5)
	assert.That(t, got["node.name"]).Equal("node-1")
	assert.That(t, got["pod.serviceAccount"]).Equal("svc-acct")
}
