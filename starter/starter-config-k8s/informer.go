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

package StarterConfigK8s

import (
	"context"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// k8sClient is the subset of the Kubernetes API used here.
type k8sClient = kubernetes.Interface

// buildClient builds a clientset: in-cluster when kubeconfig is empty, otherwise
// from the kubeconfig file.
func buildClient(kubeconfig string) (k8sClient, error) {
	var (
		cfg *rest.Config
		err error
	)
	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, errutil.Explain(err, "k8s config: load kubeconfig %q", kubeconfig)
		}
	} else {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, errutil.Explain(err, "k8s config: in-cluster config (set kubeconfig when running outside a cluster)")
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errutil.Explain(err, "k8s config: build clientset")
	}
	return client, nil
}

// watchManager tracks informers so they can be stopped on shutdown, and
// deduplicates watchers so repeated Load calls do not stack informers.
type watchManager struct {
	mu      sync.Mutex
	watched map[string]struct{}
	stops   []chan struct{}
}

// objVersion extracts an informer object's resourceVersion for refresh
// tagging. The handlers receive the concrete typed objects, which all
// implement metav1.Object; wrappers like cache.DeletedFinalStateUnknown do
// not, and yield an empty version.
func objVersion(obj any) string {
	if o, ok := obj.(metav1.Object); ok {
		return o.GetResourceVersion()
	}
	return ""
}

// ensureWatch starts a namespaced, name-scoped informer on the target object
// and triggers a full property refresh on every add/update/delete.
func (c *k8sCtrl) ensureWatch(client k8sClient, cs configSource) {
	id := fmt.Sprintf("%s/%s/%s", cs.kind, cs.namespace, cs.objectName)

	c.manager.mu.Lock()
	if _, ok := c.manager.watched[id]; ok {
		c.manager.mu.Unlock()
		return
	}
	c.manager.watched[id] = struct{}{}
	c.manager.mu.Unlock()

	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		0, // event-driven only; no periodic resync needed for a single object
		informers.WithNamespace(cs.namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.FieldSelector = "metadata.name=" + cs.objectName
		}),
	)

	var informer cache.SharedIndexInformer
	switch cs.kind {
	case kindConfigMap:
		informer = factory.Core().V1().ConfigMaps().Informer()
	case kindSecret:
		informer = factory.Core().V1().Secrets().Informer()
	default:
		return
	}

	handler := cache.ResourceEventHandlerFuncs{
		// Stamp each trigger with the change's identity (which object, which
		// resourceVersion) so the refresh records logged and metered by
		// observability.RefreshConf carry what this round is about. The
		// resourceVersion doubles as the refresh identifier: it advances on
		// every write to the object, so two refreshes from the same object are
		// distinguishable. UpdateFunc stamps the new object's version.
		AddFunc: func(obj any) {
			c.TriggerRefresh(log.WithFields(context.Background(),
				log.String("source", "k8s"),
				log.String("object", id),
				log.String("resource_version", objVersion(obj)),
			))
		},
		UpdateFunc: func(_, newObj any) {
			c.TriggerRefresh(log.WithFields(context.Background(),
				log.String("source", "k8s"),
				log.String("object", id),
				log.String("resource_version", objVersion(newObj)),
			))
		},
		DeleteFunc: func(obj any) {
			c.TriggerRefresh(log.WithFields(context.Background(),
				log.String("source", "k8s"),
				log.String("object", id),
				log.String("resource_version", objVersion(obj)),
			))
		},
	}
	if _, err := informer.AddEventHandler(handler); err != nil {
		log.Errorf(context.Background(), starterTag,
			"k8s config: add event handler for %s failed; %s will not hot-reload until a restart: %v", id, id, err)
		c.manager.forget(id)
		return
	}

	stop := make(chan struct{})
	factory.Start(stop)
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		close(stop)
		log.Errorf(context.Background(), starterTag,
			"k8s config: cache sync for %s timed out; %s will not hot-reload until a restart", id, id)
		c.manager.forget(id)
		return
	}

	c.manager.mu.Lock()
	c.manager.stops = append(c.manager.stops, stop)
	c.manager.mu.Unlock()

	log.Infof(context.Background(), starterTag, "watching k8s object kind=%s namespace=%s name=%s", cs.kind, cs.namespace, cs.objectName)
}

// forget drops a watcher id so a later Load may retry starting it.
func (m *watchManager) forget(id string) {
	m.mu.Lock()
	delete(m.watched, id)
	m.mu.Unlock()
}

// stopAll stops every running informer.
func (m *watchManager) stopAll() {
	m.mu.Lock()
	stops := m.stops
	m.stops = nil
	m.watched = map[string]struct{}{}
	m.mu.Unlock()
	for _, s := range stops {
		close(s)
	}
	if n := len(stops); n > 0 {
		log.Infof(context.Background(), starterTag, "stopped %d k8s informer(s)", n)
	}
}
