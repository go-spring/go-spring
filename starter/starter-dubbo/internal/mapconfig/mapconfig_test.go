/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mapconfig

import (
	"fmt"
	"testing"
)

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopkg.in/yaml.v3"
)

import (
	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/config_center"
	"dubbo.apache.org/dubbo-go/v3/config_center/parser"
	"dubbo.apache.org/dubbo-go/v3/registry"
	"dubbo.apache.org/dubbo-go/v3/remoting"
)

func TestNewMapDynamicConfiguration(t *testing.T) {
	dc := NewMapDynamicConfiguration()
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.Parser())
}

func TestPublishAndGet(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	err := dc.PublishConfig("test-key", constant.Dubbo, "test-value")
	require.NoError(t, err)

	val, err := dc.GetProperties("test-key", config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	assert.Equal(t, "test-value", val)

	val, err = dc.GetRule("test-key", config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	assert.Equal(t, "test-value", val)
}

func TestRemoveConfig(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	_ = dc.PublishConfig("test-key", constant.Dubbo, "test-value")
	_ = dc.RemoveConfig("test-key", constant.Dubbo)

	val, err := dc.GetProperties("test-key", config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	assert.Empty(t, val)
}

func TestGetConfigKeysByGroup(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	_ = dc.PublishConfig("key1", constant.Dubbo, "v1")
	_ = dc.PublishConfig("key2", constant.Dubbo, "v2")

	keys, err := dc.GetConfigKeysByGroup(constant.Dubbo)
	require.NoError(t, err)
	assert.True(t, keys.Contains("key1"))
	assert.True(t, keys.Contains("key2"))
}

func TestListenerNotification(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	received := make(chan *config_center.ConfigChangeEvent, 1)
	listener := &testListener{ch: received}

	dc.AddListener("my-key", listener, config_center.WithGroup(constant.Dubbo))

	_ = dc.PublishConfig("my-key", constant.Dubbo, "new-value")

	event := <-received
	assert.Equal(t, "my-key", event.Key)
	assert.Equal(t, "new-value", event.Value)
	assert.Equal(t, remoting.EventTypeAdd, event.ConfigType)
}

func TestListenerNotificationOnDelete(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	received := make(chan *config_center.ConfigChangeEvent, 1)
	listener := &testListener{ch: received}

	_ = dc.PublishConfig("my-key", constant.Dubbo, "value")
	dc.AddListener("my-key", listener, config_center.WithGroup(constant.Dubbo))

	_ = dc.RemoveConfig("my-key", constant.Dubbo)

	event := <-received
	assert.Equal(t, "my-key", event.Key)
	assert.Equal(t, remoting.EventTypeDel, event.ConfigType)
}

func TestSetOverrideRule_GeneratesValidYAML(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	dc.SetOverrideRule("my-app", map[string]string{
		"timeout":                     "5000",
		"retries":                     "3",
		"methods.GetUser.timeout":     "2000",
		"methods.GetUser.retries":     "5",
	})

	key := "my-app" + constant.ConfiguratorSuffix
	raw, err := dc.GetRule(key, config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	urls, err := dc.Parser().ParseToUrls(raw)
	require.NoError(t, err)
	require.NotEmpty(t, urls)

	url := urls[0]
	assert.Equal(t, "5000", url.GetParam("timeout", ""))
	assert.Equal(t, "3", url.GetParam("retries", ""))
	assert.Equal(t, "2000", url.GetParam("methods.GetUser.timeout", ""))
	assert.Equal(t, "5", url.GetParam("methods.GetUser.retries", ""))
}

func TestSetServiceOverrideRule_GeneratesValidYAML(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	dc.SetServiceOverrideRule("org.example.UserService:1.0.0:default", map[string]string{
		"timeout": "3000",
	})

	key := "org.example.UserService:1.0.0:default" + constant.ConfiguratorSuffix
	raw, err := dc.GetRule(key, config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	urls, err := dc.Parser().ParseToUrls(raw)
	require.NoError(t, err)
	require.NotEmpty(t, urls)

	url := urls[0]
	assert.Equal(t, "3000", url.GetParam("timeout", ""))
}

func TestDeleteOverrideRule(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	dc.SetOverrideRule("my-app", map[string]string{"timeout": "5000"})
	dc.DeleteOverrideRule("my-app")

	key := "my-app" + constant.ConfiguratorSuffix
	raw, err := dc.GetRule(key, config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	assert.Empty(t, raw)
}

func TestOverrideRule_EndToEnd_SimulateDubboFlow(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	appKey := "my-test-app" + constant.ConfiguratorSuffix
	dc.SetOverrideRule("my-test-app", map[string]string{
		"timeout": "5000",
		"retries": "3",
	})

	raw, err := dc.GetRule(appKey, config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)

	urls, err := dc.Parser().ParseToUrls(raw)
	require.NoError(t, err)
	require.NotEmpty(t, urls)

	configurators := registry.ToConfigurators(urls, func(url *common.URL) config_center.Configurator {
		return &testConfigurator{url: url}
	})

	targetURL, err := common.NewURL("dubbo://127.0.0.1:20880/org.example.UserService?timeout=1000&retries=2&side=consumer&application=my-test-app")
	require.NoError(t, err)

	assert.Equal(t, "1000", targetURL.GetParam("timeout", ""))
	assert.Equal(t, "2", targetURL.GetParam("retries", ""))

	for _, c := range configurators {
		c.Configure(targetURL)
	}

	assert.Equal(t, "5000", targetURL.GetParam("timeout", ""))
	assert.Equal(t, "3", targetURL.GetParam("retries", ""))
}

func TestOverrideRule_ListenerTriggeredOnSet(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	received := make(chan *config_center.ConfigChangeEvent, 1)
	listener := &testListener{ch: received}

	appKey := "my-app" + constant.ConfiguratorSuffix
	dc.AddListener(appKey, listener, config_center.WithGroup(constant.Dubbo))

	dc.SetOverrideRule("my-app", map[string]string{"timeout": "5000"})

	event := <-received
	assert.Equal(t, appKey, event.Key)
	assert.Equal(t, remoting.EventTypeAdd, event.ConfigType)

	var cfg parser.ConfiguratorConfig
	err := yaml.Unmarshal([]byte(event.Value.(string)), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "my-app", cfg.Key)
	assert.Equal(t, "5000", cfg.Configs[0].Parameters["timeout"])
}

func TestRefreshOverrideRules_AddAndUpdate(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	dc.RefreshOverrideRules(map[string]map[string]string{
		"my-app": {
			"timeout": "5000",
			"retries": "3",
		},
		"org.example.UserService:1.0.0:default": {
			"timeout": "3000",
		},
	})

	appKey := "my-app" + constant.ConfiguratorSuffix
	raw, err := dc.GetRule(appKey, config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	urls, err := dc.Parser().ParseToUrls(raw)
	require.NoError(t, err)
	assert.Equal(t, "5000", urls[0].GetParam("timeout", ""))

	svcKey := "org.example.UserService:1.0.0:default" + constant.ConfiguratorSuffix
	raw, err = dc.GetRule(svcKey, config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	urls, err = dc.Parser().ParseToUrls(raw)
	require.NoError(t, err)
	assert.Equal(t, "3000", urls[0].GetParam("timeout", ""))
}

func TestRefreshOverrideRules_DeleteRemovedKeys(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	dc.RefreshOverrideRules(map[string]map[string]string{
		"app-a": {"timeout": "5000"},
		"app-b": {"timeout": "3000"},
	})

	dc.RefreshOverrideRules(map[string]map[string]string{
		"app-a": {"timeout": "8000"},
	})

	raw, err := dc.GetRule("app-a"+constant.ConfiguratorSuffix, config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	assert.Contains(t, raw, "8000")

	raw, err = dc.GetRule("app-b"+constant.ConfiguratorSuffix, config_center.WithGroup(constant.Dubbo))
	require.NoError(t, err)
	assert.Empty(t, raw)
}

func TestRefreshOverrideRules_NotifiesListeners(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	addCh := make(chan *config_center.ConfigChangeEvent, 4)
	delCh := make(chan *config_center.ConfigChangeEvent, 4)

	appAKey := "app-a" + constant.ConfiguratorSuffix
	appBKey := "app-b" + constant.ConfiguratorSuffix

	dc.AddListener(appAKey, &testListener{ch: addCh}, config_center.WithGroup(constant.Dubbo))
	dc.AddListener(appBKey, &testListener{ch: delCh}, config_center.WithGroup(constant.Dubbo))

	dc.RefreshOverrideRules(map[string]map[string]string{
		"app-a": {"timeout": "5000"},
		"app-b": {"timeout": "3000"},
	})

	evtA := <-addCh
	assert.Equal(t, remoting.EventTypeAdd, evtA.ConfigType)
	evtB := <-delCh
	assert.Equal(t, remoting.EventTypeAdd, evtB.ConfigType)

	dc.RefreshOverrideRules(map[string]map[string]string{
		"app-a": {"timeout": "8000"},
	})

	evtA2 := <-addCh
	assert.Equal(t, remoting.EventTypeUpdate, evtA2.ConfigType)
	assert.Contains(t, evtA2.Value, "8000")

	evtB2 := <-delCh
	assert.Equal(t, remoting.EventTypeDel, evtB2.ConfigType)
}

func TestRefreshOverrideRules_EmptySnapshotClearsAll(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	dc.RefreshOverrideRules(map[string]map[string]string{
		"app-a": {"timeout": "5000"},
		"app-b": {"timeout": "3000"},
	})

	dc.RefreshOverrideRules(map[string]map[string]string{})

	keys, err := dc.GetConfigKeysByGroup(constant.Dubbo)
	require.NoError(t, err)
	assert.Equal(t, 0, keys.Size())
}

func TestSnapshot(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	_ = dc.PublishConfig("k1", constant.Dubbo, "v1")
	_ = dc.PublishConfig("k2", constant.Dubbo, "v2")
	_ = dc.PublishConfig("k3", "other-group", "v3")

	snap := dc.Snapshot()
	assert.Len(t, snap, 2)
	assert.Equal(t, "v1", snap[constant.Dubbo]["k1"])
	assert.Equal(t, "v2", snap[constant.Dubbo]["k2"])
	assert.Equal(t, "v3", snap["other-group"]["k3"])
}

func TestSnapshotKeys(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	dc.AddListener("key-a", &testListener{}, config_center.WithGroup(constant.Dubbo))
	dc.AddListener("key-b", &testListener{}, config_center.WithGroup(constant.Dubbo))

	keys := dc.SnapshotKeys()
	assert.Len(t, keys, 2)
	assert.Contains(t, keys[0], "key-a")
	assert.Contains(t, keys[1], "key-b")
}

func TestListenerPanicRecovery(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	received := make(chan *config_center.ConfigChangeEvent, 2)

	// listener that panics
	dc.AddListener("key", &panicListener{}, config_center.WithGroup(constant.Dubbo))
	// listener that should still work after the first one panics
	dc.AddListener("key", &testListener{ch: received}, config_center.WithGroup(constant.Dubbo))

	// this should not panic, and the second listener should still receive the event
	_ = dc.PublishConfig("key", constant.Dubbo, "value")

	event := <-received
	assert.Equal(t, "value", event.Value)
}

func TestConcurrentPublishAndGet(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	const goroutines = 100
	done := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			key := fmt.Sprintf("key-%d", id%10)
			_ = dc.PublishConfig(key, constant.Dubbo, fmt.Sprintf("v-%d", id))
			dc.GetProperties(key, config_center.WithGroup(constant.Dubbo))
			dc.GetRule(key, config_center.WithGroup(constant.Dubbo))
			dc.GetConfigKeysByGroup(constant.Dubbo)
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}
}

func TestConcurrentRefreshAndGet(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	done := make(chan struct{}, 50)

	// writers: refresh every iteration
	for i := 0; i < 20; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				dc.RefreshOverrideRules(map[string]map[string]string{
					fmt.Sprintf("app-%d", j%5): {"timeout": fmt.Sprintf("%d", j)},
				})
			}
		}(i)
	}

	// readers: concurrent gets
	for i := 0; i < 30; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				dc.GetRule("app-0"+constant.ConfiguratorSuffix, config_center.WithGroup(constant.Dubbo))
				dc.Snapshot()
				dc.SnapshotKeys()
			}
		}()
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestConcurrentAddListenerAndNotify(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	done := make(chan struct{}, 100)
	received := make(chan struct{}, 1000)

	for i := 0; i < 50; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			key := fmt.Sprintf("key-%d", id%5)
			dc.AddListener(key, &countListener{ch: received}, config_center.WithGroup(constant.Dubbo))
		}(i)
		go func(id int) {
			defer func() { done <- struct{}{} }()
			key := fmt.Sprintf("key-%d", id%5)
			_ = dc.PublishConfig(key, constant.Dubbo, fmt.Sprintf("v-%d", id))
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestRefreshOverrideRules_ConcurrentReaders(t *testing.T) {
	dc := NewMapDynamicConfiguration()

	dc.RefreshOverrideRules(map[string]map[string]string{
		"app-a": {"timeout": "1000"},
		"app-b": {"timeout": "2000"},
	})

	done := make(chan struct{}, 20)
	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				raw, err := dc.GetRule("app-a"+constant.ConfiguratorSuffix, config_center.WithGroup(constant.Dubbo))
				require.NoError(t, err)
				assert.NotEmpty(t, raw)
			}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

// ---------------------------------------------------------------
// test helpers
// ---------------------------------------------------------------
type testListener struct {
	ch chan *config_center.ConfigChangeEvent
}

func (l *testListener) Process(event *config_center.ConfigChangeEvent) {
	if l.ch != nil {
		l.ch <- event
	}
}

type countListener struct {
	ch chan struct{}
}

func (l *countListener) Process(event *config_center.ConfigChangeEvent) {
	if l.ch != nil {
		l.ch <- struct{}{}
	}
}

type panicListener struct{}

func (l *panicListener) Process(event *config_center.ConfigChangeEvent) {
	panic("test panic")
}

type testConfigurator struct {
	url *common.URL
}

func (c *testConfigurator) GetUrl() *common.URL {
	return c.url
}

func (c *testConfigurator) Configure(url *common.URL) {
	url.SetParams(c.url.GetParams())
}