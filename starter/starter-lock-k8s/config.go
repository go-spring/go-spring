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

package StarterLockK8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"go-spring.org/stdlib/errutil"
)

// Config binds one Kubernetes-Lease-backed distributed-lock instance under
// "${spring.lock.<name>}". Every field has a default so a minimal in-cluster
// configuration needs no keys at all; the ServiceAccount config and the
// "default" namespace are used.
//
// Lock timing (TTL, renew, retry) is not configured here: it is carried by the
// per-acquire [lock.Option] values (and their defaults), so the same knobs work
// identically across every lock backend.
type Config struct {
	// Namespace is the Kubernetes namespace the Lease objects live in. The
	// application's ServiceAccount needs get/create/update RBAC on
	// coordination.k8s.io/leases in this namespace.
	Namespace string `value:"${namespace:=default}"`

	// Kubeconfig is the path to a kubeconfig file used when running outside a
	// cluster (local development, tests). When empty the in-cluster
	// ServiceAccount config is used.
	Kubeconfig string `value:"${kubeconfig:=}"`

	// KeyPrefix is prepended to every lock key to form the Lease object name so
	// multiple applications can share one namespace without colliding. The
	// resulting name must be a valid DNS-1123 subdomain (lowercase alphanumerics,
	// '-' and '.'); pick keys accordingly.
	KeyPrefix string `value:"${key-prefix:=}"`
}

// buildClient builds a Kubernetes clientset for c: in-cluster when Kubeconfig
// is empty, otherwise from the kubeconfig file. It is built eagerly at startup
// so a missing ServiceAccount or bad kubeconfig fails fast.
func buildClient(c Config) (kubernetes.Interface, error) {
	restCfg, err := buildRESTConfig(c)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, errutil.Explain(err, "lock-k8s: build clientset")
	}
	return client, nil
}

// buildRESTConfig selects in-cluster config or an explicit kubeconfig file.
func buildRESTConfig(c Config) (*rest.Config, error) {
	if c.Kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", c.Kubeconfig)
		if err != nil {
			return nil, errutil.Explain(err, "lock-k8s: load kubeconfig %q", c.Kubeconfig)
		}
		return cfg, nil
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, errutil.Explain(err, "lock-k8s: in-cluster config (set kubeconfig when running outside a cluster)")
	}
	return cfg, nil
}
