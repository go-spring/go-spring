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

// Package podinfo exposes Kubernetes Pod metadata — name, namespace, IP, node,
// service account, and labels — to the application, with zero third-party
// dependencies.
//
// It does not talk to the Kubernetes API. Instead it relies on the Downward API:
// the Deployment injects Pod fields as environment variables (name, namespace,
// IP, ...) and mounts labels/annotations as a file. Go-Spring's config layer
// maps GS_-prefixed environment variables into the property tree (GS_POD_NAME ->
// pod.name), so the struct fields below bind straight from configuration.
//
// The struct carries `value` tags but imports nothing from the IoC container, so
// it stays in the zero-dependency stdlib layer. To use it, register it as a bean
// in the application and autowire it:
//
//	gs.Object(&podinfo.PodInfo{})
//
//	type MyService struct {
//	    Pod *podinfo.PodInfo `autowire:""`
//	}
//
// The `gs k8s` scaffolding generates a Deployment that wires the matching
// Downward API environment variables and the labels volume, plus a k8s config
// profile that sets pod.labels.path.
package podinfo

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// PodInfo holds Kubernetes Pod metadata bound from configuration. Every field
// defaults to empty, so an application running outside Kubernetes (where the
// Downward API variables are absent) simply sees zero values rather than
// failing to wire.
type PodInfo struct {
	// Name is the Pod name (metadata.name), injected as GS_POD_NAME.
	Name string `value:"${pod.name:=}"`
	// Namespace is the Pod namespace (metadata.namespace), injected as
	// GS_POD_NAMESPACE.
	Namespace string `value:"${pod.namespace:=}"`
	// IP is the Pod IP (status.podIP), injected as GS_POD_IP.
	IP string `value:"${pod.ip:=}"`
	// NodeName is the host node name (spec.nodeName), injected as GS_NODE_NAME.
	NodeName string `value:"${node.name:=}"`
	// ServiceAccount is the Pod service account (spec.serviceAccountName),
	// injected as GS_POD_SERVICE_ACCOUNT.
	ServiceAccount string `value:"${pod.service.account:=}"`
	// LabelsPath is the mount path of the Downward API labels file (e.g.
	// /etc/podinfo/labels). Empty when no labels volume is mounted.
	LabelsPath string `value:"${pod.labels.path:=}"`
}

// Metadata returns the non-empty scalar fields as a map, suitable as a source of
// service-discovery registration metadata. LabelsPath is excluded — it is an
// implementation detail, not metadata worth publishing.
func (p *PodInfo) Metadata() map[string]string {
	m := make(map[string]string, 5)
	for k, v := range map[string]string{
		"pod.name":           p.Name,
		"pod.namespace":      p.Namespace,
		"pod.ip":             p.IP,
		"node.name":          p.NodeName,
		"pod.serviceAccount": p.ServiceAccount,
	} {
		if v != "" {
			m[k] = v
		}
	}
	return m
}

// Labels reads and parses the Downward API labels file at LabelsPath. Kubernetes
// writes one entry per line in the form key="value", where the value is a
// double-quoted Go/JSON string literal. When LabelsPath is empty (no labels
// volume mounted), it returns an empty map and no error.
func (p *PodInfo) Labels() (map[string]string, error) {
	labels := map[string]string{}
	if p.LabelsPath == "" {
		return labels, nil
	}
	f, err := os.Open(p.LabelsPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		key, quoted, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val, err := strconv.Unquote(strings.TrimSpace(quoted))
		if err != nil {
			// Fall back to the raw value if it is not a quoted literal, so a
			// malformed line degrades rather than aborting the whole parse.
			val = strings.Trim(strings.TrimSpace(quoted), `"`)
		}
		labels[key] = val
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return labels, nil
}
