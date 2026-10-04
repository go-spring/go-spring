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

// Package StarterConfigK8s integrates a Kubernetes ConfigMap or Secret as a
// hot-reloadable configuration source, read directly through the API server
// rather than through a mounted volume. Blank-importing this package registers a
// "k8s" config provider that can be consumed via
// spring.config.import, together with the bridge that wires API-driven changes
// into the application-wide property refresh for live hot-reload.
//
// It complements starter-config-file. The file starter watches a volume mount
// and inherits the kubelet's projection latency (~1min for Secret rotation);
// this starter opens a client-go informer straight onto the ConfigMap/Secret,
// so a `kubectl edit configmap` propagates to bound gs.Dync fields within
// seconds and can target objects in any namespace the ServiceAccount may read.
// Pick file mode for zero RBAC, API mode for immediacy and cross-namespace
// reach.
package StarterConfigK8s

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register the controller itself, not just its Load method: the runtime
	// holds the registered provider so it can stop the informers at shutdown.
	// Refreshes go through the gs.RefreshProperties
	// package-level facade, so the controller needs no bean wiring at all.
	conf.RegisterProvider("k8s", newK8sCtrl())
}

var starterTag = log.RegisterAppTag("config_k8s", "")

// watchSyncTimeout bounds how long Load waits for a new informer's initial
// cache sync before giving up on hot-reload for that object.
var watchSyncTimeout = 30 * time.Second

// k8sCtrl is the single object that owns the full lifecycle of k8s
// configuration: loading ConfigMaps/Secrets, watching via informers, and
// triggering property refresh.
type k8sCtrl struct {
	// manager tracks informers so they can be stopped on shutdown.
	manager *watchManager

	// clientMu guards clients, the clientset cache keyed by kubeconfig path.
	// Load runs on every property refresh; without the cache each refresh
	// would build a fresh clientset (re-reading the in-cluster config, and
	// leaving the old connection pool behind) instead of reusing one.
	clientMu sync.Mutex
	clients  map[string]k8sClient

	onTrigger func() // test hook; nil in production
}

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

// newK8sCtrl creates a controller with its watch manager and client cache
// ready, so neither ensureWatch nor clientFor needs a lazy nil-check.
func newK8sCtrl() *k8sCtrl {
	return &k8sCtrl{
		manager: &watchManager{watched: map[string]struct{}{}},
		clients: map[string]k8sClient{},
	}
}

// sourceFields returns the fields identifying a watched object. They are
// attached to a context with log.WithFields rather than repeated at each call
// site; the informer event handlers stamp the same object under "object" when
// they trigger a refresh.
func sourceFields(cs configSource) []log.Field {
	return []log.Field{
		log.String("kind", cs.kind),
		log.String("namespace", cs.namespace),
		log.String("name", cs.objectName),
	}
}

// clientFor returns a cached clientset for the kubeconfig path, creating one
// if necessary. The cache is what keeps repeated refreshes from leaking a
// clientset's connection pool per Load call.
func (c *k8sCtrl) clientFor(kubeconfig string) (k8sClient, error) {
	c.clientMu.Lock()
	defer c.clientMu.Unlock()

	if cli, ok := c.clients[kubeconfig]; ok {
		return cli, nil
	}
	cli, err := buildClient(kubeconfig)
	if err != nil {
		return nil, err
	}
	c.clients[kubeconfig] = cli
	return cli, nil
}

// TriggerRefresh is called by the informer event handlers when a watched
// ConfigMap or Secret changes. Before the app has started,
// gs.RefreshProperties returns an error and the change is dropped — the
// initial config load already captured the state.
func (c *k8sCtrl) TriggerRefresh(ctx context.Context) {
	if c.onTrigger != nil {
		c.onTrigger()
		return
	}
	// The refresh outcome (status, duration, error) is logged and metered
	// centrally by observability.RefreshConf; this layer only records backend events.
	_ = observability.RefreshConf(ctx, gs.RefreshProperties)
}

// Close tears down every informer. It is the provider lifecycle hook, invoked
// once by the runtime on shutdown. A Load that follows re-arms: the watch
// manager forgets its watcher ids, so ensureWatch starts fresh informers.
func (c *k8sCtrl) Close(ctx context.Context) error {
	if c.manager != nil {
		c.manager.stopAll(ctx)
	}
	return nil
}

// Object kinds accepted in a provider source.
const (
	kindConfigMap = "configmap"
	kindSecret    = "secret"
)

// "k8s" is registered as a configuration provider in starter.go's init, as
// the controller instance itself (so the runtime can close it), so that a
// spring.config.import entry such as
//
//	optional:k8s:configmap/app-config?namespace=default&key=application.yaml
//
// loads configuration straight from the named ConfigMap/Secret at startup
// and, whenever the object changes, triggers a full property refresh. This is
// the piece that makes an API-watched ConfigMap/Secret hot-reloadable: the
// informer fires on every add/update/delete and turns that into a refresh,
// without waiting on kubelet volume projection.

// configSource holds the parsed components of a "k8s" provider source.
type configSource struct {
	kind       string // "configmap" or "secret"
	objectName string // object name
	namespace  string // object namespace
	dataKey    string // when set, only this data entry is read
	format     string // format override applied to every read entry
	kubeconfig string // kubeconfig path; empty means in-cluster
}

// parseSource parses a provider source of the form
//
//	<kind>/<name>[?namespace=..&key=..&format=..&kubeconfig=..]
//
// The leading "k8s:" prefix has already been stripped by conf/provider.Load.
func parseSource(source string) (configSource, error) {
	cs := configSource{namespace: "default"}
	path := source
	if p, query, ok := strings.Cut(source, "?"); ok {
		path = p
		q, err := url.ParseQuery(query)
		if err != nil {
			return configSource{}, errutil.Explain(err, "invalid k8s config query in %q", source)
		}
		if v := q.Get("namespace"); v != "" {
			cs.namespace = v
		}
		cs.dataKey = q.Get("key")
		cs.format = q.Get("format")
		cs.kubeconfig = q.Get("kubeconfig")
	}

	kind, name, ok := strings.Cut(path, "/")
	if !ok || name == "" {
		return configSource{}, errutil.Explain(nil, "k8s config source %q must be <kind>/<name>", source)
	}
	cs.kind = strings.ToLower(kind)
	cs.objectName = name
	if cs.kind != kindConfigMap && cs.kind != kindSecret {
		return configSource{}, errutil.Explain(nil, "unsupported k8s config kind %q (want %q or %q)", kind, kindConfigMap, kindSecret)
	}
	if cs.format != "" {
		if !reader.Has(cs.format) {
			return configSource{}, errutil.Explain(nil, "unsupported k8s config format %q", cs.format)
		}
	}
	return cs, nil
}

// Load implements conf/provider.Provider. It reads the target ConfigMap/Secret
// through the API server, parses its data entries, and installs an informer
// that triggers an application property refresh on change.
func (c *k8sCtrl) Load(optional bool, source string) (map[string]string, error) {
	// Load is the top of this chain and Provider.Load takes no context, so the
	// one ctx the whole load shares is minted here — the helpers below receive
	// it instead of each minting their own.
	ctx := context.Background()

	cs, err := parseSource(source)
	if err != nil {
		log.Error(ctx, starterTag,
			log.String("source", source),
			log.Err(err),
			log.Msg("parse k8s source failed"))
		return nil, err
	}

	// The source's identity rides on the context from here on: every event this
	// load and its helpers print carries it without repeating it at each call
	// site.
	ctx = log.WithFields(ctx, sourceFields(cs)...)

	log.Debug(ctx, starterTag, func() []log.Field {
		return []log.Field{
			log.String("key", cs.dataKey),
			log.String("format", cs.format),
			log.Msg("loading k8s config"),
		}
	})

	client, err := c.clientFor(cs.kubeconfig)
	if err != nil {
		if optional {
			log.Warn(ctx, starterTag,
				log.Err(err),
				log.Msg("optional config build client failed, skipped"))
			return nil, nil
		}
		log.Error(ctx, starterTag,
			log.Err(err),
			log.Msg("build k8s client failed"))
		return nil, err
	}
	return c.loadFromClient(ctx, client, cs, optional)
}

// loadFromClient reads and parses the object through client and installs the
// change watcher. It is the seam tests use to inject a client-go fake clientset
// instead of a live cluster.
func (c *k8sCtrl) loadFromClient(ctx context.Context, client k8sClient, cs configSource, optional bool) (map[string]string, error) {
	data, err := fetch(ctx, client, cs)
	if err != nil {
		if apierrors.IsNotFound(err) && optional {
			log.Warn(ctx, starterTag,
				log.Msg("optional config not found, skipped"))
			return nil, nil
		}
		log.Error(ctx, starterTag,
			log.Err(err),
			log.Msg("fetch k8s object failed"))
		return nil, err
	}

	// Install the informer before returning so a change that lands right after
	// the initial read is not missed. Watching is best-effort: a failure only
	// loses hot-reload for this object, the static snapshot still loads.
	c.ensureWatch(ctx, client, cs)

	m := map[string]string{}
	if err := parseEntries(cs, data, m); err != nil {
		return nil, err
	}

	log.Info(ctx, starterTag,
		log.Int("keys", len(m)),
		log.Msg("loaded k8s config"))
	return m, nil
}

// fetch reads the raw data entries of the target object as name -> bytes. For a
// Secret both Data (already decoded) is used; for a ConfigMap both Data (string)
// and BinaryData are merged.
func fetch(ctx context.Context, client k8sClient, cs configSource) (map[string][]byte, error) {
	switch cs.kind {
	case kindConfigMap:
		cm, err := client.CoreV1().ConfigMaps(cs.namespace).Get(ctx, cs.objectName, metav1.GetOptions{})
		if err != nil {
			return nil, errutil.Explain(err, "k8s config: get configmap %s/%s", cs.namespace, cs.objectName)
		}
		return configMapData(cm), nil
	case kindSecret:
		sec, err := client.CoreV1().Secrets(cs.namespace).Get(ctx, cs.objectName, metav1.GetOptions{})
		if err != nil {
			return nil, errutil.Explain(err, "k8s config: get secret %s/%s", cs.namespace, cs.objectName)
		}
		return sec.Data, nil
	default:
		return nil, errutil.Explain(nil, "k8s config: unsupported kind %q", cs.kind)
	}
}

// configMapData merges a ConfigMap's string Data and BinaryData into one
// name -> bytes map.
func configMapData(cm *corev1.ConfigMap) map[string][]byte {
	out := make(map[string][]byte, len(cm.Data)+len(cm.BinaryData))
	for k, v := range cm.Data {
		out[k] = []byte(v)
	}
	maps.Copy(out, cm.BinaryData)
	return out
}

// parseEntries parses each selected data entry as a config document and merges
// its flattened keys into m. Each entry name (e.g. "application.yaml") supplies
// the format by extension unless a format override is set. When key is set only
// that entry is read; entries with an unknown extension and no forced format are
// skipped, mirroring the file starter's directory semantics.
func parseEntries(cs configSource, data map[string][]byte, m map[string]string) error {
	for name, content := range data {
		if cs.dataKey != "" && name != cs.dataKey {
			continue
		}
		format := cs.format
		if format == "" {
			ext := name
			if i := strings.LastIndex(name, "."); i >= 0 {
				ext = name[i+1:]
			}
			if !reader.Has(ext) {
				if cs.dataKey != "" {
					return errutil.Explain(nil, "k8s config: entry %q has no known format; set format=", name)
				}
				continue
			}
			format = ext
		}
		parsed, err := reader.Read(format, content)
		if err != nil {
			return errutil.Explain(err, "k8s config: parse entry %q", name)
		}
		maps.Copy(m, flatten.Flatten(parsed))
	}
	return nil
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
func (c *k8sCtrl) ensureWatch(ctx context.Context, client k8sClient, cs configSource) {
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
		log.Error(ctx, starterTag,
			log.Err(err),
			log.Msg("k8s config: adding the event handler failed; it will not hot-reload until a restart"))
		c.manager.forget(id)
		return
	}

	stop := make(chan struct{})
	factory.Start(stop)
	// Bound the sync wait: WaitForCacheSync alone would block Load (and thus
	// app startup) forever when the API server keeps failing the initial LIST
	// (e.g. RBAC missing the watch verb) — the reflector retries indefinitely
	// and the cache never syncs.
	synced := make(chan bool, 1)
	go func() { synced <- cache.WaitForCacheSync(stop, informer.HasSynced) }()
	select {
	case ok := <-synced:
		if !ok {
			close(stop)
			log.Error(ctx, starterTag,
				log.Msg("k8s config: watch stopped before cache sync; it will not hot-reload until a restart"))
			c.manager.forget(id)
			return
		}
	case <-time.After(watchSyncTimeout):
		close(stop)
		log.Error(ctx, starterTag,
			log.String("timeout", watchSyncTimeout.String()),
			log.Msg("k8s config: cache sync timed out; it will not hot-reload until a restart"))
		c.manager.forget(id)
		return
	}

	c.manager.mu.Lock()
	c.manager.stops = append(c.manager.stops, stop)
	c.manager.mu.Unlock()

	log.Info(ctx, starterTag, log.Msg("watching k8s object for changes"))
}

// forget drops a watcher id so a later Load may retry starting it.
func (m *watchManager) forget(id string) {
	m.mu.Lock()
	delete(m.watched, id)
	m.mu.Unlock()
}

// stopAll stops every running informer.
func (m *watchManager) stopAll(ctx context.Context) {
	m.mu.Lock()
	stops := m.stops
	m.stops = nil
	m.watched = map[string]struct{}{}
	m.mu.Unlock()
	for _, s := range stops {
		close(s)
	}
	if n := len(stops); n > 0 {
		log.Infof(ctx, starterTag, "stopped %d k8s informer(s)", n)
	}
}
