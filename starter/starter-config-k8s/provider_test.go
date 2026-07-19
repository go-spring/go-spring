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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

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
	assert.String(t, cs.name).Equal("app")
	assert.String(t, cs.namespace).Equal("prod")
	assert.String(t, cs.key).Equal("application.yaml")
	assert.String(t, cs.format).Equal("yaml")

	// Namespace defaults to "default".
	cs, err = parseSource("secret/creds")
	assert.Error(t, err).Nil()
	assert.String(t, cs.kind).Equal(kindSecret)
	assert.String(t, cs.namespace).Equal("default")

	// Bad kind and malformed path are rejected up front.
	_, err = parseSource("deployment/x")
	assert.Error(t, err).Matches("unsupported k8s config kind")
	_, err = parseSource("configmap")
	assert.Error(t, err).Matches("must be <kind>/<name>")
	_, err = parseSource("configmap/x?format=xml")
	assert.Error(t, err).Matches("unsupported k8s config format")
}

func TestLoadConfigMapYAML(t *testing.T) {
	client := fake.NewSimpleClientset(configMap("cm-yaml", map[string]string{
		"application.yaml": "server:\n  port: 8080\nname: demo\n",
	}))
	cs, err := parseSource("configmap/cm-yaml")
	assert.Error(t, err).Nil()

	m, err := loadFromClient(client, cs, false)
	assert.Error(t, err).Nil()
	assert.String(t, m["server.port"]).Equal("8080")
	assert.String(t, m["name"]).Equal("demo")
	manager.stopAll()
}

func TestLoadSecretPropsWithKeyFilter(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "sec", Namespace: "default"},
		Data: map[string][]byte{
			"db.properties":    []byte("db.user=root\ndb.pass=secret\n"),
			"other.properties": []byte("ignore.me=1\n"),
		},
	})
	cs, err := parseSource("secret/sec?key=db.properties")
	assert.Error(t, err).Nil()

	m, err := loadFromClient(client, cs, false)
	assert.Error(t, err).Nil()
	assert.String(t, m["db.user"]).Equal("root")
	assert.String(t, m["db.pass"]).Equal("secret")
	// The key filter excluded the other entry.
	_, ok := m["ignore.me"]
	assert.That(t, ok).False()
	manager.stopAll()
}

func TestLoadOptionalMissing(t *testing.T) {
	client := fake.NewSimpleClientset()
	cs, err := parseSource("configmap/absent")
	assert.Error(t, err).Nil()

	// Optional + not found returns an empty snapshot, not an error.
	m, err := loadFromClient(client, cs, true)
	assert.Error(t, err).Nil()
	assert.That(t, m == nil).True()

	// Required + not found is an error.
	_, err = loadFromClient(client, cs, false)
	assert.Error(t, err).Matches("get configmap")
}

func TestUnknownExtensionSkippedButKeyFilterErrors(t *testing.T) {
	client := fake.NewSimpleClientset(configMap("mixed", map[string]string{
		"application.yaml": "a: 1\n",
		"README":           "not config",
	}))
	// Without a key filter, the extension-less entry is silently skipped.
	cs, err := parseSource("configmap/mixed")
	assert.Error(t, err).Nil()
	m, err := loadFromClient(client, cs, false)
	assert.Error(t, err).Nil()
	assert.String(t, m["a"]).Equal("1")
	manager.stopAll()

	// Selecting the extension-less entry with no forced format is an error.
	cs, err = parseSource("configmap/mixed?key=README")
	assert.Error(t, err).Nil()
	_, err = loadFromClient(client, cs, false)
	assert.Error(t, err).Matches("no known format")
	manager.stopAll()
}

func TestHotReloadTriggersRefresh(t *testing.T) {
	client := fake.NewSimpleClientset(configMap("live", map[string]string{
		"application.yaml": "v: 1\n",
	}))
	cs, err := parseSource("configmap/live")
	assert.Error(t, err).Nil()

	fired := make(chan struct{}, 8)
	setRefreshHook(func() error { fired <- struct{}{}; return nil })
	defer setRefreshHook(nil)

	_, err = loadFromClient(client, cs, false)
	assert.Error(t, err).Nil()
	defer manager.stopAll()

	// The initial informer sync fires the Add handler; drain it, then update the
	// ConfigMap and expect a refresh from the Update handler.
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

// drain removes any buffered signals so a subsequent wait observes only new ones.
func drain(ch chan struct{}) {
	for {
		select {
		case <-ch:
		case <-time.After(200 * time.Millisecond):
			return
		}
	}
}
