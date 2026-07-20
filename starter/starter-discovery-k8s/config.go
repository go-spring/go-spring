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

// Package StarterDiscoveryK8s integrates Kubernetes-native service discovery
// into Go-Spring's client-side discovery abstraction (stdlib/discovery).
//
// Inside a cluster the platform already registers every Pod behind a Service,
// so an application should discover peers through that platform capability
// rather than stand up a second external registry (Nacos/Consul). This starter
// registers one or more discovery.Discovery backends — selected by name from a
// client starter's `discovery:` field — that resolve a Kubernetes Service name
// to the set of live Pod endpoints.
//
// Two modes are offered (see DESIGN / README):
//
//   - dns: resolves a headless Service's DNS SRV/A records. Zero extra
//     dependency, no RBAC, but DNS caching slows change propagation and there
//     is no per-endpoint metadata.
//   - endpointslice: watches the target Service's EndpointSlices through a
//     client-go informer. Real-time and carries Pod metadata (zone, ready
//     state), at the cost of a client-go dependency and get/list/watch RBAC on
//     endpointslices.
package StarterDiscoveryK8s

import (
	"time"

	"go-spring.org/spring/discovery"
	"go-spring.org/stdlib/errutil"
)

// Mode names for Config.Mode.
const (
	ModeDNS           = "dns"
	ModeEndpointSlice = "endpointslice"
)

// Config binds one Kubernetes discovery backend under
// "${spring.discovery.k8s.<name>}". The map key <name> is the backend name a
// client starter references via its `discovery:` field (e.g. redis
// `discovery: k8s`). The service name passed to Resolve/Watch is the
// Kubernetes Service name; this Config supplies the surrounding context
// (namespace, port selection, auth) that turns it into live endpoints.
type Config struct {
	// Mode selects the discovery mechanism: "dns" (default, headless Service
	// DNS, zero dependency) or "endpointslice" (client-go informer, real-time).
	Mode string `value:"${mode:=dns}"`

	// Namespace is the Kubernetes namespace the target Service lives in.
	Namespace string `value:"${namespace:=default}"`

	// PortName selects which named Service port to connect to.
	//   - dns mode: when set, an SRV query on "_<port-name>._tcp.<svc>..."
	//     yields both address and port; when empty an A query is used and the
	//     Port field supplies the port.
	//   - endpointslice mode: matches EndpointSlice port entries by name; when
	//     empty the Port field (or the slice's sole port) is used.
	PortName string `value:"${port-name:=}"`

	// Port is the numeric port used when PortName is empty. Required for dns
	// A-record mode (records carry no port); optional in endpointslice mode.
	Port int `value:"${port:=0}"`

	// ClusterDomain is the cluster DNS suffix used to build the Service FQDN in
	// dns mode. It has no effect in endpointslice mode.
	ClusterDomain string `value:"${cluster-domain:=cluster.local}"`

	// RefreshInterval is how often the dns-mode watcher re-resolves the Service
	// to detect endpoint changes (DNS has no push channel). Ignored in
	// endpointslice mode, which is driven by informer events.
	RefreshInterval time.Duration `value:"${refresh-interval:=10s}"`

	// Kubeconfig is the path to a kubeconfig file for endpointslice mode. When
	// empty the in-cluster ServiceAccount config is used. Ignored in dns mode.
	Kubeconfig string `value:"${kubeconfig:=}"`

	// ResyncPeriod is the informer resync period in endpointslice mode. Zero
	// disables periodic resync (event-driven only). Ignored in dns mode.
	ResyncPeriod time.Duration `value:"${resync-period:=0}"`
}

// validate rejects a Config whose required fields for the chosen mode are
// missing, so a misconfiguration fails at startup rather than on first resolve.
func (c Config) validate() error {
	switch c.Mode {
	case ModeDNS:
		if c.PortName == "" && c.Port <= 0 {
			return errutil.Explain(nil, "discovery-k8s: dns mode requires port-name (SRV) or port (A record)")
		}
	case ModeEndpointSlice:
		// No hard requirement: PortName/Port narrow port selection, and empty
		// Kubeconfig means in-cluster. Port selection falls back to the slice's
		// sole port at resolve time.
	default:
		return errutil.Explain(nil, "discovery-k8s: invalid mode %q (want %q or %q)", c.Mode, ModeDNS, ModeEndpointSlice)
	}
	return nil
}

// newBackend builds the discovery.Discovery backend for the given Config.
func newBackend(c Config) (discovery.Discovery, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	switch c.Mode {
	case ModeDNS:
		return newDNSDiscovery(c, nil), nil
	case ModeEndpointSlice:
		return newEndpointSliceDiscovery(c)
	default:
		// Unreachable: validate already rejected unknown modes.
		return nil, errutil.Explain(nil, "discovery-k8s: invalid mode %q", c.Mode)
	}
}
