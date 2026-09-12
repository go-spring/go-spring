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

// Package StarterGovernanceNacos adapts Nacos as a governance rule source: it
// listens on one Nacos dataId holding the governance rule document and pushes
// every published version into the governance center through the
// [go-spring.org/cloud/governance.Source] contract.
//
// It is the Nacos member of the governance-source adapter family — the
// Sentinel-datasource shape. The rule document is governance's own, not an
// application-config import, so enabling governance never turns on app-property
// refresh and vice versa; this module is therefore separate from
// starter-config-nacos even though both speak to Nacos. The source owns its
// client exclusively, so it is free to carry its own instrumentation
// independently of the config-import bootstrap client.
//
// Configure with ${govern.source.nacos.*}:
//
//	govern.source.nacos.server=127.0.0.1:8848
//	govern.source.nacos.data-id=app-govern.yaml
//	govern.source.nacos.group=DEFAULT_GROUP
//
// The bean is registered only when that key is present, so importing this
// package is inert otherwise.
package StarterGovernanceNacos

import (
	"context"
	"reflect"
	"strconv"
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

var starterTag = log.RegisterAppTag("governance_nacos", "")

// dialTimeoutMs bounds the source's own client calls.
const dialTimeoutMs = 5000

// governNacosConfig binds ${govern.source.nacos.*}.
type governNacosConfig struct {
	// Server is the Nacos server address, host:port.
	Server string `value:"${server}" expr:"$ != ''"`

	// DataID is the dataId holding the rules document.
	DataID string `value:"${data-id}" expr:"$ != ''"`

	// Group and Namespace locate the dataId; Group defaults to DEFAULT_GROUP.
	Group     string `value:"${group:=DEFAULT_GROUP}"`
	Namespace string `value:"${namespace:=}"`

	// Username and Password authenticate against Nacos; empty means no auth.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

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
		src := governSource{
			dataID: c.DataID,
			group:  c.Group,
			format: sourceFormat(c.Format, c.DataID),
		}

		r.Provide(func() (*NacosSource, error) {
			host, port, err := splitHostPort(c.Server)
			if err != nil {
				return nil, err
			}
			cli, err := clients.NewConfigClient(vo.NacosClientParam{
				ClientConfig: constant.NewClientConfig(
					constant.WithNamespaceId(c.Namespace),
					constant.WithTimeoutMs(dialTimeoutMs),
					constant.WithUsername(c.Username),
					constant.WithPassword(c.Password),
					constant.WithNotLoadCacheAtStart(true),
				),
				ServerConfigs: []constant.ServerConfig{*constant.NewServerConfig(host, port)},
			})
			if err != nil {
				return nil, errutil.Explain(err, "governance nacos source: create client for %s failed", c.Server)
			}
			return NewNacosSource(cli, src)
		}).
			Init((*NacosSource).Init).Destroy((*NacosSource).Close).
			Export(gs.As[governance.Source]()).Caller(1)
		return nil
	})
}

// sourceFormat resolves the rule document format: an explicit format wins, else
// the name's extension, else properties.
func sourceFormat(explicit, name string) string {
	if explicit != "" {
		return explicit
	}
	if ext := strings.TrimPrefix(extOf(name), "."); ext != "" {
		return ext
	}
	return "properties"
}

// extOf returns the dotted extension of name ("" when none).
func extOf(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i:]
	}
	return ""
}

// splitHostPort splits a "host:port" Nacos server address.
func splitHostPort(server string) (string, uint64, error) {
	host, portStr, ok := strings.Cut(server, ":")
	if !ok || host == "" || portStr == "" {
		return "", 0, errutil.Explain(nil, "nacos server address must be host:port, got %q", server)
	}
	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		return "", 0, errutil.Explain(err, "invalid nacos server port in %q", server)
	}
	return host, port, nil
}

// governSource is the resolved wiring one NacosSource carries: which dataId to
// read and how to parse it.
type governSource struct {
	dataID string
	group  string
	format string
}

// NacosSource is a governance.Source backed by one nacos dataId. The initial
// GetConfig seeds the snapshot (a missing or bad document fails construction,
// so a misconfigured dataId surfaces at startup); ListenConfig then delivers
// each published version, which re-parses through governance.ParseRules and
// pushes when the rules actually changed. A bad publish keeps the last good
// snapshot and logs.
type NacosSource struct {
	cli config_client.IConfigClient
	src governSource
	doc string // latest document bytes (for dedupe before parsing)

	mu  sync.Mutex
	cfg governance.Config
	cb  func(governance.Config)
}

// NewNacosSource seeds the snapshot from the dataId's current content.
func NewNacosSource(cli config_client.IConfigClient, src governSource) (*NacosSource, error) {
	content, err := cli.GetConfig(vo.ConfigParam{DataId: src.dataID, Group: src.group})
	if err != nil {
		return nil, errutil.Explain(err, "governance nacos source: get %s/%s failed", src.group, src.dataID)
	}
	cfg, err := rules.Parse(src.dataID, []byte(content), src.format)
	if err != nil {
		return nil, err
	}
	return &NacosSource{cli: cli, src: src, doc: content, cfg: cfg}, nil
}

// Init installs the change listener (the gs bean lifecycle hook).
func (s *NacosSource) Init() error {
	return s.cli.ListenConfig(vo.ConfigParam{
		DataId: s.src.dataID,
		Group:  s.src.group,
		OnChange: func(namespace, group, dataId, data string) {
			s.apply(data)
		},
	})
}

// Close removes the listener and closes the source's own client — the client
// is built by the bean ctor and dies with the bean. Implements the
// optional-close contract the governance center probes for on Destroy.
func (s *NacosSource) Close() error {
	err := s.cli.CancelListenConfig(vo.ConfigParam{DataId: s.src.dataID, Group: s.src.group})
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
// swaps the snapshot and pushes. Byte-equal re-deliveries (nacos may re-push on
// reconnect) and bad documents push nothing.
func (s *NacosSource) apply(data string) {
	if data == s.doc {
		return
	}
	cfg, err := rules.Parse(s.src.dataID, []byte(data), s.src.format)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "governance nacos source: %s/%s published an invalid document (keeping last good config): %v", s.src.group, s.src.dataID, err)
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
