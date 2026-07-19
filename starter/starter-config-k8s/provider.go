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
// "k8s" config provider (see provider.go) that can be consumed via
// spring.app.imports, together with the bridge that wires API-driven changes
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
	"maps"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// Object kinds accepted in a provider source.
const (
	kindConfigMap = "configmap"
	kindSecret    = "secret"
)

func init() {
	// Register "k8s" as a configuration provider so that a spring.app.imports
	// entry such as
	//
	//	optional:k8s:configmap/app-config?namespace=default&key=application.yaml
	//
	// loads configuration straight from the named ConfigMap/Secret at startup
	// and, whenever the object changes, triggers a full property refresh. This is
	// the piece that makes an API-watched ConfigMap/Secret hot-reloadable: the
	// informer (see informer.go) fires on every add/update/delete and turns that
	// into a refresh, without waiting on kubelet volume projection.
	conf.RegisterProvider("k8s", loadK8sConfig)
}

// contentReader parses raw configuration bytes into a nested map based on a
// declared format name. Used both for the forced "format" query and for the
// per-entry format inferred from a ConfigMap/Secret data key's extension.
type contentReader func(b []byte) (map[string]any, error)

var contentReaders = map[string]contentReader{
	"properties": prop.Read,
	"props":      prop.Read,
	"yaml":       yaml.Read,
	"yml":        yaml.Read,
	"toml":       toml.Read,
	"tml":        toml.Read,
	"json":       json.Read,
}

// configSource holds the parsed components of a "k8s" provider source.
type configSource struct {
	kind       string // "configmap" or "secret"
	name       string // object name
	namespace  string // object namespace
	key        string // when set, only this data entry is read
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
		cs.key = q.Get("key")
		cs.format = q.Get("format")
		cs.kubeconfig = q.Get("kubeconfig")
	}

	kind, name, ok := strings.Cut(path, "/")
	if !ok || name == "" {
		return configSource{}, errutil.Explain(nil, "k8s config source %q must be <kind>/<name>", source)
	}
	cs.kind = strings.ToLower(kind)
	cs.name = name
	if cs.kind != kindConfigMap && cs.kind != kindSecret {
		return configSource{}, errutil.Explain(nil, "unsupported k8s config kind %q (want %q or %q)", kind, kindConfigMap, kindSecret)
	}
	if cs.format != "" {
		if _, ok := contentReaders[cs.format]; !ok {
			return configSource{}, errutil.Explain(nil, "unsupported k8s config format %q", cs.format)
		}
	}
	return cs, nil
}

// loadK8sConfig implements conf/provider.Provider. It reads the target
// ConfigMap/Secret through the API server, parses its data entries, and installs
// an informer that triggers an application property refresh on change.
func loadK8sConfig(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}

	client, err := buildClient(cs.kubeconfig)
	if err != nil {
		if optional {
			return nil, nil
		}
		return nil, err
	}
	return loadFromClient(client, cs, optional)
}

// loadFromClient reads and parses the object through client and installs the
// change watcher. It is the seam tests use to inject a client-go fake clientset
// instead of a live cluster.
func loadFromClient(client k8sClient, cs configSource, optional bool) (map[string]string, error) {
	ctx := context.Background()
	data, err := fetch(ctx, client, cs)
	if err != nil {
		if apierrors.IsNotFound(err) && optional {
			return nil, nil
		}
		return nil, err
	}

	// Install the informer before returning so a change that lands right after
	// the initial read is not missed. Watching is best-effort: a failure only
	// loses hot-reload for this object, the static snapshot still loads.
	ensureWatch(client, cs)

	m := map[string]string{}
	if err := parseEntries(cs, data, m); err != nil {
		return nil, err
	}
	return m, nil
}

// fetch reads the raw data entries of the target object as name -> bytes. For a
// Secret both Data (already decoded) is used; for a ConfigMap both Data (string)
// and BinaryData are merged.
func fetch(ctx context.Context, client k8sClient, cs configSource) (map[string][]byte, error) {
	switch cs.kind {
	case kindConfigMap:
		cm, err := client.CoreV1().ConfigMaps(cs.namespace).Get(ctx, cs.name, metav1.GetOptions{})
		if err != nil {
			return nil, errutil.Explain(err, "k8s config: get configmap %s/%s", cs.namespace, cs.name)
		}
		return configMapData(cm), nil
	case kindSecret:
		sec, err := client.CoreV1().Secrets(cs.namespace).Get(ctx, cs.name, metav1.GetOptions{})
		if err != nil {
			return nil, errutil.Explain(err, "k8s config: get secret %s/%s", cs.namespace, cs.name)
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
		if cs.key != "" && name != cs.key {
			continue
		}
		format := cs.format
		if format == "" {
			ext := name
			if i := strings.LastIndex(name, "."); i >= 0 {
				ext = name[i+1:]
			}
			if _, ok := contentReaders[ext]; !ok {
				if cs.key != "" {
					return errutil.Explain(nil, "k8s config: entry %q has no known format; set format=", name)
				}
				continue
			}
			format = ext
		}
		parsed, err := contentReaders[format](content)
		if err != nil {
			return errutil.Explain(err, "k8s config: parse entry %q", name)
		}
		maps.Copy(m, flatten.Flatten(parsed))
	}
	return nil
}
