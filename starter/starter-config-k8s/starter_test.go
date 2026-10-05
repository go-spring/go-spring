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
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"go-spring.org/stdlib/testing/assert"
)

func configMap(name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Data:       data,
	}
}

func TestParseSource(t *testing.T) {
	cs, err := parseSource("configmap/app?namespace=prod&key=application.yaml&format=yaml")
	assert.Error(t, err).Nil()
	assert.String(t, cs.kind).Equal(kindConfigMap)
	assert.String(t, cs.objectName).Equal("app")
	assert.String(t, cs.namespace).Equal("prod")
	assert.String(t, cs.dataKey).Equal("application.yaml")
	assert.String(t, cs.format).Equal("yaml")

	cs, err = parseSource("secret/creds")
	assert.Error(t, err).Nil()
	assert.String(t, cs.kind).Equal(kindSecret)
	assert.String(t, cs.namespace).Equal("default")

	_, err = parseSource("deployment/x")
	assert.Error(t, err).Matches("unsupported k8s config kind")
	_, err = parseSource("configmap")
	assert.Error(t, err).Matches("must be <kind>/<name>")
	_, err = parseSource("configmap/x?format=xml")
	assert.Error(t, err).Matches("unsupported k8s config format")
}

func TestLoadConfigMapYAML(t *testing.T) {
	c := newK8sCtrl()

	client := fake.NewSimpleClientset(configMap("cm-yaml", map[string]string{
		"application.yaml": "server:\n  port: 8080\nname: demo\n",
	}))
	cs, err := parseSource("configmap/cm-yaml")
	assert.Error(t, err).Nil()

	m, err := c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Nil()
	assert.String(t, m["server.port"]).Equal("8080")
	assert.String(t, m["name"]).Equal("demo")
	c.manager.stopAll()
}

func TestLoadSecretPropsWithKeyFilter(t *testing.T) {
	c := newK8sCtrl()

	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "sec", Namespace: "default"},
		Data: map[string][]byte{
			"db.properties":    []byte("db.user=root\ndb.pass=secret\n"),
			"other.properties": []byte("ignore.me=1\n"),
		},
	})
	cs, err := parseSource("secret/sec?key=db.properties")
	assert.Error(t, err).Nil()

	m, err := c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Nil()
	assert.String(t, m["db.user"]).Equal("root")
	assert.String(t, m["db.pass"]).Equal("secret")
	_, ok := m["ignore.me"]
	assert.That(t, ok).False()
	c.manager.stopAll()
}

func TestLoadOptionalMissing(t *testing.T) {
	c := newK8sCtrl()

	client := fake.NewSimpleClientset()
	cs, err := parseSource("configmap/absent")
	assert.Error(t, err).Nil()

	m, err := c.loadFromClient(context.Background(), client, cs, true)
	assert.Error(t, err).Nil()
	assert.That(t, m == nil).True()

	_, err = c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Matches("get configmap")
}

func TestUnknownExtensionSkippedButKeyFilterErrors(t *testing.T) {
	c := newK8sCtrl()

	client := fake.NewSimpleClientset(configMap("mixed", map[string]string{
		"application.yaml": "a: 1\n",
		"README":           "not config",
	}))
	cs, err := parseSource("configmap/mixed")
	assert.Error(t, err).Nil()
	m, err := c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Nil()
	assert.String(t, m["a"]).Equal("1")
	c.manager.stopAll()

	cs, err = parseSource("configmap/mixed?key=README")
	assert.Error(t, err).Nil()
	_, err = c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Matches("no known format")
	c.manager.stopAll()
}

func TestHotReloadTriggersRefresh(t *testing.T) {
	client := fake.NewSimpleClientset(configMap("live", map[string]string{
		"application.yaml": "v: 1\n",
	}))
	cs, err := parseSource("configmap/live")
	assert.Error(t, err).Nil()

	fired := make(chan struct{}, 8)
	c := newK8sCtrl()
	c.onTrigger = func() { fired <- struct{}{} }

	_, err = c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Nil()
	defer c.manager.stopAll()

	drain(fired)
	_, err = client.CoreV1().ConfigMaps("default").Update(context.Background(),
		configMap("live", map[string]string{"application.yaml": "v: 2\n"}), metav1.UpdateOptions{})
	assert.Error(t, err).Nil()

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("no refresh fired on ConfigMap update")
	}
}

func drain(ch chan struct{}) {
	for {
		select {
		case <-ch:
		case <-time.After(200 * time.Millisecond):
			return
		}
	}
}

func TestLoadBadKubeconfigOptionalSkips(t *testing.T) {
	c := newK8sCtrl()

	// optional: a client that cannot be built is skipped with no error.
	m, err := c.Load(context.Background(), true, "configmap/x?kubeconfig=/nonexistent/kubeconfig")
	assert.Error(t, err).Nil()
	assert.That(t, m == nil).True()

	// required: the same failure is fatal.
	_, err = c.Load(context.Background(), false, "configmap/x?kubeconfig=/nonexistent/kubeconfig")
	assert.Error(t, err).Matches("load kubeconfig")

	// parse failures are fatal regardless of optional.
	_, err = c.Load(context.Background(), true, "deployment/x")
	assert.Error(t, err).Matches("unsupported k8s config kind")
}

func TestClientForCachesByKubeconfig(t *testing.T) {
	dir := t.TempDir()
	kc := filepath.Join(dir, "kubeconfig")
	err := os.WriteFile(kc, []byte(`apiVersion: v1
kind: Config
clusters:
- cluster: {server: "http://127.0.0.1:1"}
  name: c
contexts:
- context: {cluster: c, user: u}
  name: ctx
current-context: ctx
users:
- name: u
  user: {}
`), 0o600)
	assert.Error(t, err).Nil()

	c := newK8sCtrl()
	a, err := c.clientFor(context.Background(), kc)
	assert.Error(t, err).Nil()
	b, err := c.clientFor(context.Background(), kc)
	assert.Error(t, err).Nil()
	assert.That(t, a == b).True() // cached, not rebuilt per Load

	// failures are not cached: the map stays empty.
	_, err = c.clientFor(context.Background(), "/nonexistent/kubeconfig")
	assert.Error(t, err).Matches("load kubeconfig")
	_, err = c.clientFor(context.Background(), "/nonexistent/kubeconfig")
	assert.Error(t, err).Matches("load kubeconfig")
}

func TestEnsureWatchDedupAndRearmAfterClose(t *testing.T) {
	c := newK8sCtrl()
	client := fake.NewSimpleClientset(configMap("dedup", map[string]string{"a.properties": "a=1\n"}))
	cs, err := parseSource("configmap/dedup")
	assert.Error(t, err).Nil()

	_, err = c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Nil()
	_, err = c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Nil()

	c.manager.mu.Lock()
	watched, stops := len(c.manager.watched), len(c.manager.stops)
	c.manager.mu.Unlock()
	assert.That(t, watched == 1).True() // repeated Load does not stack informers
	assert.That(t, stops == 1).True()

	// Close forgets everything; a later Load re-arms a fresh informer.
	c.Close()
	c.manager.mu.Lock()
	watched = len(c.manager.watched)
	c.manager.mu.Unlock()
	assert.That(t, watched == 0).True()

	_, err = c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Nil()
	c.manager.stopAll()
}

func TestWatchSyncTimeoutForgetsAndStillLoads(t *testing.T) {
	old := watchSyncTimeout
	watchSyncTimeout = 100 * time.Millisecond
	defer func() { watchSyncTimeout = old }()

	c := newK8sCtrl()
	client := fake.NewSimpleClientset(configMap("stuck", map[string]string{"a.properties": "a=1\n"}))
	// Make the informer's initial LIST fail forever, so the cache never syncs.
	client.PrependReactor("list", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "configmaps"}, "", nil)
	})
	cs, err := parseSource("configmap/stuck")
	assert.Error(t, err).Nil()

	// The static snapshot still loads; the watch gives up after the timeout.
	m, err := c.loadFromClient(context.Background(), client, cs, false)
	assert.Error(t, err).Nil()
	assert.String(t, m["a"]).Equal("1")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.manager.mu.Lock()
		n := len(c.manager.watched)
		c.manager.mu.Unlock()
		if n == 0 {
			return // forgotten: a later Load may retry the watch
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("watched id not forgotten after cache-sync timeout")
}

func TestObjVersion(t *testing.T) {
	cm := configMap("rv", nil)
	cm.ResourceVersion = "42"
	assert.String(t, objVersion(cm)).Equal("42")
	assert.String(t, objVersion("not-an-object")).Equal("")
}

func TestFetchUnsupportedKind(t *testing.T) {
	client := fake.NewSimpleClientset()
	cs := configSource{kind: "deployment", namespace: "default", objectName: "x"}
	_, err := fetch(context.Background(), client, cs)
	assert.Error(t, err).Matches("unsupported kind")
}
