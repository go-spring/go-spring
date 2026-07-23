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

// Package mapconfig provides an in-memory DynamicConfiguration implementation.
// It is intended for environments where configuration is managed programmatically
// (e.g. through an internal platform API, admin endpoints, or unit tests) rather
// than through an external config center.
//
// Usage:
//
//	import (
//	    "dubbo.apache.org/dubbo-go/v3/common/config"
//	    mapconfig "go-spring.org/starter-dubbo/internal/mapconfig"
//	)
//
//	dc := mapconfig.NewMapDynamicConfiguration()
//	config.GetEnvInstance().SetDynamicConfiguration(dc)
//
//	dc.SetOverrideRule("my-app", map[string]string{
//	    "timeout": "5000",
//	    "retries": "3",
//	    "methods.GetUser.timeout": "2000",
//	    "methods.GetUser.retries": "5",
//	})
package mapconfig

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"dubbo.apache.org/dubbo-go/v3/common"
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/common/extension"
	"dubbo.apache.org/dubbo-go/v3/config_center"
	"dubbo.apache.org/dubbo-go/v3/config_center/parser"
	"dubbo.apache.org/dubbo-go/v3/remoting"
	gxset "github.com/dubbogo/gost/container/set"
	"github.com/dubbogo/gost/log/logger"
	"gopkg.in/yaml.v3"
)

// singleton is the process-wide MapDynamicConfiguration instance. Both the
// gs bean (provided via Singleton) and the dubbo-go extension factory
// (GetDynamicConfiguration) return this same instance, so the poller
// and dubbo-go's configurator listener operate on the same data.
var singleton = NewMapDynamicConfiguration()

// Singleton returns the process-wide MapDynamicConfiguration instance.
func Singleton() *MapDynamicConfiguration {
	return singleton
}

// configCenterType is the name under which this DynamicConfiguration factory
// is registered in dubbo-go's extension registry. Callers that need to use
// this in-memory config center set dubbo.config-center.protocol="map".
const configCenterType = "map"

func init() {
	extension.SetConfigCenterFactory(configCenterType, func() config_center.DynamicConfigurationFactory {
		return &mapDynamicConfigurationFactory{}
	})
}

type mapDynamicConfigurationFactory struct{}

func (f *mapDynamicConfigurationFactory) GetDynamicConfiguration(_ *common.URL) (config_center.DynamicConfiguration, error) {
	return Singleton(), nil
}

// ---------------------------------------------------------------
// MapDynamicConfiguration
// ---------------------------------------------------------------

// MapDynamicConfiguration is an in-memory DynamicConfiguration.
//
// Data is organised as: group → key → value.
// The default group "dubbo" holds override rules and properties.
//
// It is safe for concurrent use.
type MapDynamicConfiguration struct {
	config_center.BaseDynamicConfiguration
	parser parser.ConfigurationParser

	mu        sync.RWMutex
	data      map[string]map[string]string // group -> key -> value
	listeners map[listenerKey][]config_center.ConfigurationListener
}

type listenerKey struct {
	Group string
	Key   string
}

// NewMapDynamicConfiguration creates a ready-to-use MapDynamicConfiguration.
func NewMapDynamicConfiguration() *MapDynamicConfiguration {
	dc := &MapDynamicConfiguration{
		data:      make(map[string]map[string]string),
		listeners: make(map[listenerKey][]config_center.ConfigurationListener),
	}
	dc.SetParser(&parser.DefaultConfigurationParser{})
	return dc
}

// ---------------------------------------------------------------
// DynamicConfiguration interface
// ---------------------------------------------------------------

func (dc *MapDynamicConfiguration) Parser() parser.ConfigurationParser {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.parser
}
func (dc *MapDynamicConfiguration) SetParser(p parser.ConfigurationParser) {
	dc.mu.Lock()
	dc.parser = p
	dc.mu.Unlock()
}

func (dc *MapDynamicConfiguration) AddListener(key string, listener config_center.ConfigurationListener, opts ...config_center.Option) {
	tmpOpts := config_center.NewOptions(opts...)
	lk := listenerKey{Group: tmpOpts.Center.Group, Key: key}

	dc.mu.Lock()
	dc.listeners[lk] = append(dc.listeners[lk], listener)
	dc.mu.Unlock()
}

func (dc *MapDynamicConfiguration) RemoveListener(key string, listener config_center.ConfigurationListener, opts ...config_center.Option) {
	tmpOpts := config_center.NewOptions(opts...)
	lk := listenerKey{Group: tmpOpts.Center.Group, Key: key}

	dc.mu.Lock()
	listeners := dc.listeners[lk]
	for i, l := range listeners {
		if l == listener {
			dc.listeners[lk] = append(listeners[:i], listeners[i+1:]...)
			break
		}
	}
	if len(dc.listeners[lk]) == 0 {
		delete(dc.listeners, lk)
	}
	dc.mu.Unlock()
}

func (dc *MapDynamicConfiguration) GetProperties(key string, opts ...config_center.Option) (string, error) {
	return dc.lookup(key, opts...), nil
}

func (dc *MapDynamicConfiguration) GetRule(key string, opts ...config_center.Option) (string, error) {
	return dc.lookup(key, opts...), nil
}

func (dc *MapDynamicConfiguration) GetInternalProperty(key string, opts ...config_center.Option) (string, error) {
	return dc.lookup(key, opts...), nil
}

func (dc *MapDynamicConfiguration) PublishConfig(key string, group string, value string) error {
	dc.mu.Lock()
	dc.upsertGroup(group)[key] = value
	dc.mu.Unlock()

	dc.notify(key, group, value, remoting.EventTypeAdd)
	return nil
}

func (dc *MapDynamicConfiguration) RemoveConfig(key string, group string) error {
	dc.mu.Lock()
	if g, ok := dc.data[group]; ok {
		delete(g, key)
		if len(g) == 0 {
			delete(dc.data, group)
		}
	}
	dc.mu.Unlock()

	dc.notify(key, group, "", remoting.EventTypeDel)
	return nil
}

func (dc *MapDynamicConfiguration) GetConfigKeysByGroup(group string) (*gxset.HashSet, error) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	s := gxset.NewSet()
	if g, ok := dc.data[group]; ok {
		for k := range g {
			s.Add(k)
		}
	}
	return s, nil
}

// ---------------------------------------------------------------
// Convenience API — override management
// ---------------------------------------------------------------

// SetOverrideRule pushes an application-level override rule.
//
//	dc.SetOverrideRule("my-app", map[string]string{
//	    "timeout":                     "5000",
//	    "retries":                     "3",
//	    "methods.GetUser.timeout":     "2000",
//	    "methods.GetUser.retries":     "5",
//	})
func (dc *MapDynamicConfiguration) SetOverrideRule(app string, params map[string]string) {
	dc.publishOverrideRule(app, parser.ScopeApplication, params)
}

// SetServiceOverrideRule pushes a service-level override rule.
//
//	dc.SetServiceOverrideRule("org.example.UserService:1.0.0:default", map[string]string{
//	    "timeout": "3000",
//	})
func (dc *MapDynamicConfiguration) SetServiceOverrideRule(serviceKey string, params map[string]string) {
	dc.publishOverrideRule(serviceKey, parser.GeneralType, params)
}

// DeleteOverrideRule removes an application-level override rule.
func (dc *MapDynamicConfiguration) DeleteOverrideRule(app string) {
	_ = dc.RemoveConfig(app+constant.ConfiguratorSuffix, constant.Dubbo)
}

// DeleteServiceOverrideRule removes a service-level override rule.
func (dc *MapDynamicConfiguration) DeleteServiceOverrideRule(serviceKey string) {
	_ = dc.RemoveConfig(serviceKey+constant.ConfiguratorSuffix, constant.Dubbo)
}

// RefreshOverrideRules replaces all override rules atomically with the given snapshot.
// rules is keyed by app name or service key (colon-separated). Each value is a
// flat map of dubbo URL parameters (e.g. "timeout", "retries", "methods.X.timeout").
//
// Keys absent in the snapshot are deleted (listeners get EventTypeDel);
// new or changed keys are updated (listeners get EventTypeUpdate / EventTypeAdd).
//
// Passing nil clears all override rules.
func (dc *MapDynamicConfiguration) RefreshOverrideRules(rules map[string]map[string]string) {
	dc.mu.Lock()

	oldKeys := dc.snapshotKeys()

	newGroup := make(map[string]string, len(rules))
	for name, params := range rules {
		key := name + constant.ConfiguratorSuffix
		scope := scopeForName(name)
		newGroup[key] = marshalOverrideRule(name, scope, params)
	}
	dc.data[constant.Dubbo] = newGroup

	notifications := diffNotifications(oldKeys, newGroup)
	dc.mu.Unlock()

	for _, n := range notifications {
		dc.notify(n.key, constant.Dubbo, n.value, n.eventType)
	}
}

// Snapshot returns a read-only copy of all stored configuration.
// Intended for debugging and operations tooling.
func (dc *MapDynamicConfiguration) Snapshot() map[string]map[string]string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	snap := make(map[string]map[string]string, len(dc.data))
	for g, group := range dc.data {
		entries := make(map[string]string, len(group))
		for k, v := range group {
			entries[k] = v
		}
		snap[g] = entries
	}
	return snap
}

// SnapshotKeys returns the sorted list of all listener keys currently registered.
func (dc *MapDynamicConfiguration) SnapshotKeys() []string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	keys := make([]string, 0, len(dc.listeners))
	for lk := range dc.listeners {
		keys = append(keys, fmt.Sprintf("group=%q key=%q", lk.Group, lk.Key))
	}
	sort.Strings(keys)
	return keys
}

// ---------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------

func (dc *MapDynamicConfiguration) upsertGroup(group string) map[string]string {
	g, ok := dc.data[group]
	if !ok {
		g = make(map[string]string)
		dc.data[group] = g
	}
	return g
}

func (dc *MapDynamicConfiguration) lookup(key string, opts ...config_center.Option) string {
	tmpOpts := config_center.NewOptions(opts...)

	dc.mu.RLock()
	defer dc.mu.RUnlock()

	if g, ok := dc.data[tmpOpts.Center.Group]; ok {
		return g[key]
	}
	return ""
}

func (dc *MapDynamicConfiguration) publishOverrideRule(name string, scope string, params map[string]string) {
	key := name + constant.ConfiguratorSuffix
	raw := marshalOverrideRule(name, scope, params)
	_ = dc.PublishConfig(key, constant.Dubbo, raw)
}

func (dc *MapDynamicConfiguration) snapshotKeys() map[string]bool {
	oldKeys := make(map[string]bool)
	if g, ok := dc.data[constant.Dubbo]; ok {
		for k := range g {
			oldKeys[k] = true
		}
	}
	return oldKeys
}

func (dc *MapDynamicConfiguration) notify(key, group, value string, eventType remoting.EventType) {
	lk := listenerKey{Group: group, Key: key}

	dc.mu.RLock()
	listeners := dc.listeners[lk]
	dc.mu.RUnlock()

	for _, l := range listeners {
		dc.safeProcess(l, key, value, eventType)
	}
}

func (dc *MapDynamicConfiguration) safeProcess(listener config_center.ConfigurationListener, key, value string, eventType remoting.EventType) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("[MapConfig] listener panic for key=%s: %v", key, r)
		}
	}()
	listener.Process(&config_center.ConfigChangeEvent{
		Key:        key,
		Value:      value,
		ConfigType: eventType,
	})
}

func marshalOverrideRule(name string, scope string, params map[string]string) string {
	cfg := parser.ConfiguratorConfig{
		ConfigVersion: "v2.7.1",
		Scope:         scope,
		Key:           name,
		Enabled:       true,
		Configs: []parser.ConfigItem{
			{
				Type:       parser.GeneralType,
				Enabled:    true,
				Addresses:  []string{constant.AnyHostValue},
				Side:       "consumer",
				Parameters: cloneParams(params),
			},
		},
	}
	raw, _ := yaml.Marshal(cfg)
	return string(raw)
}

type notification struct {
	key       string
	value     string
	eventType remoting.EventType
}

func diffNotifications(oldKeys map[string]bool, newKeys map[string]string) []notification {
	notifications := make([]notification, 0, len(oldKeys)+len(newKeys))

	for k := range oldKeys {
		if _, ok := newKeys[k]; !ok {
			notifications = append(notifications, notification{k, "", remoting.EventTypeDel})
		}
	}
	for k, v := range newKeys {
		if oldKeys[k] {
			notifications = append(notifications, notification{k, v, remoting.EventTypeUpdate})
		} else {
			notifications = append(notifications, notification{k, v, remoting.EventTypeAdd})
		}
	}
	return notifications
}

func cloneParams(params map[string]string) map[string]string {
	cp := make(map[string]string, len(params))
	for k, v := range params {
		cp[k] = v
	}
	return cp
}

// scopeForName returns ScopeApplication for a plain app name and GeneralType
// for a colon-separated service key (interface:version:group).
func scopeForName(name string) string {
	if strings.ContainsRune(name, ':') {
		return parser.GeneralType
	}
	return parser.ScopeApplication
}