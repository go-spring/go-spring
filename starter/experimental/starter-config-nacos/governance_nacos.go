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

package StarterConfigNacos

import (
	"context"
	"reflect"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/governance"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/starter-governance/rules"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// This file adds a DIRECT nacos listener for governance rules — the
// Sentinel-datasource shape: governance holds a dedicated dataId (its own
// config document, not a shared app-config import), and rule pushes refresh
// governance only, never the whole application's properties. The source owns
// its client exclusively (built here, closed on Destroy) — fully isolated
// from the config-import bootstrap client, and free to carry its own
// instrumentation. Document parsing goes through governance.ParseRules, so
// the dataId's content is byte-compatible with a starter-governance file
// source's rules file.
//
// Configure with (govern.source.nacos.*):
//
//	govern.source.nacos.server=127.0.0.1:8848
//	govern.source.nacos.data-id=app-govern.yaml
//	govern.source.nacos.group=DEFAULT_GROUP

// governNacosConfig binds ${govern.source.nacos.*}.
type governNacosConfig struct {
	Server    string `value:"${server}" expr:"$ != ''"`
	DataID    string `value:"${data-id}" expr:"$ != ''"`
	Group     string `value:"${group:=DEFAULT_GROUP}"`
	Namespace string `value:"${namespace:=}"`
	Username  string `value:"${username:=}"`
	Password  string `value:"${password:=}"`
	// Format overrides document-format detection; by default it is inferred
	// from the dataId's extension, else properties.
	Format string `value:"${format:=}"`
}

func init() {
	gs.Module(gs.OnProperty("govern.source.nacos"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c governNacosConfig
		if err := conf.Bind(p, &c, "${govern.source.nacos:=}"); err != nil {
			return err
		}

		cs := configSource{
			server:    c.Server,
			dataID:    c.DataID,
			group:     c.Group,
			namespace: c.Namespace,
			username:  c.Username,
			password:  c.Password,
			format:    c.Format,
			timeoutMs: 5000,
		}
		if cs.format == "" {
			if ext := strings.TrimPrefix(extOf(c.DataID), "."); ext != "" {
				cs.format = ext
			} else {
				cs.format = "properties"
			}
		}

		r.Provide(func() (*NacosSource, error) {
			host, port, err := splitHostPort(cs.server)
			if err != nil {
				return nil, err
			}
			cli, err := clients.NewConfigClient(vo.NacosClientParam{
				ClientConfig: constant.NewClientConfig(
					constant.WithNamespaceId(cs.namespace),
					constant.WithTimeoutMs(cs.timeoutMs),
					constant.WithUsername(cs.username),
					constant.WithPassword(cs.password),
					constant.WithNotLoadCacheAtStart(true),
				),
				ServerConfigs: []constant.ServerConfig{*constant.NewServerConfig(host, port)},
			})
			if err != nil {
				return nil, errutil.Explain(err, "create nacos config client for %s failed", cs.server)
			}
			return NewNacosSource(cli, cs)
		}).
			Init((*NacosSource).Init).Destroy((*NacosSource).Close).
			Export(gs.As[governance.Source]()).Caller(1)
		return nil
	})
}

// extOf returns the dotted extension of name ("" when none) without importing
// path/filepath just for this.
func extOf(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i:]
	}
	return ""
}

// NacosSource is a governance.Source backed by one nacos dataId. The initial
// GetConfig seeds the snapshot (a missing or bad document fails construction,
// so a misconfigured dataId surfaces at startup); ListenConfig then delivers
// each published version, which re-parses through governance.ParseRules and
// pushes when the rules actually changed. A bad publish keeps the last good
// snapshot and logs.
type NacosSource struct {
	cli config_client.IConfigClient
	cs  configSource
	doc string // latest document bytes (for dedupe before parsing)

	mu  sync.Mutex
	cfg governance.Config
	cb  func(governance.Config)
}

// NewNacosSource seeds the snapshot from the dataId's current content.
func NewNacosSource(cli config_client.IConfigClient, cs configSource) (*NacosSource, error) {
	content, err := cli.GetConfig(vo.ConfigParam{DataId: cs.dataID, Group: cs.group})
	if err != nil {
		return nil, errutil.Explain(err, "governance nacos source: get %s/%s failed", cs.group, cs.dataID)
	}
	cfg, err := rules.Parse(cs.dataID, []byte(content), cs.format)
	if err != nil {
		return nil, err
	}
	return &NacosSource{cli: cli, cs: cs, doc: content, cfg: cfg}, nil
}

// Init installs the change listener (the gs bean lifecycle hook).
func (s *NacosSource) Init() error {
	return s.cli.ListenConfig(vo.ConfigParam{
		DataId: s.cs.dataID,
		Group:  s.cs.group,
		OnChange: func(namespace, group, dataId, data string) {
			s.apply(data)
		},
	})
}

// Close removes the listener and closes the source's own client — the
// client is built by the bean ctor and dies with the bean, fully isolated
// from the config-import bootstrap client. Implements the optional-close
// contract the governance center probes for on Destroy.
func (s *NacosSource) Close() error {
	err := s.cli.CancelListenConfig(vo.ConfigParam{DataId: s.cs.dataID, Group: s.cs.group})
	s.cli.CloseClient()
	return err
}

// Snapshot returns the latest good snapshot.
func (s *NacosSource) Snapshot() governance.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Subscribe registers cb as the push target (the center is the only consumer).
func (s *NacosSource) Subscribe(cb func(governance.Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cb = cb
}

// apply parses one delivered document and, when the rules actually changed,
// swaps the snapshot and pushes. Byte-equal re-deliveries (nacos may re-push
// on reconnect) and bad documents push nothing.
func (s *NacosSource) apply(data string) {
	if data == s.doc {
		return
	}
	cfg, err := rules.Parse(s.cs.dataID, []byte(data), s.cs.format)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "governance nacos source: %s/%s published an invalid document (keeping last good config): %v", s.cs.group, s.cs.dataID, err)
		return
	}

	s.mu.Lock()
	unchanged := reflect.DeepEqual(s.cfg, cfg)
	s.cfg, s.doc = cfg, data
	cb := s.cb
	s.mu.Unlock()

	if unchanged {
		return
	}
	if cb != nil {
		cb(cfg)
	}
}
