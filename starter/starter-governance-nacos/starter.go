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
// Configure with ${spring.governance.source.nacos.*}:
//
//	spring.governance.source.nacos.server=127.0.0.1:8848
//	spring.governance.source.nacos.data-id=app-governance.yaml
//	spring.governance.source.nacos.group=DEFAULT_GROUP
//
// The bean is registered only when that key is present, so importing this
// package is inert otherwise.
package StarterGovernanceNacos

import (
	"context"
	"reflect"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/governance"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/netutil"
	"go-spring.org/stdlib/randutil"
)

var starterTag = log.RegisterAppTag("governance", "nacos")

func init() {
	gs.Module(gs.OnProperty("spring.governance.source.nacos"), func(r gs.BeanProvider, _ flatten.Storage) error {
		r.Provide(func(c governanceNacosConfig) (*NacosSource, error) {
			return NewNacosSource(c)
		}, gs.TagArg("${spring.governance.source.nacos:=}")).
			Init((*NacosSource).Init).
			Destroy((*NacosSource).Close).
			Export(gs.As[governance.Source]()).
			Caller(1)
		return nil
	})
}

// governanceSource is the resolved wiring one NacosSource carries: which dataId to
// read and how to parse it.
type governanceSource struct {
	dataID string
	group  string
	format string
}

// NacosSource is a governance.Source backed by one nacos dataId. The initial
// GetConfig seeds the snapshot (a missing or bad document fails construction,
// so a misconfigured dataId surfaces at startup); ListenConfig then delivers
// each published version, which re-parses through governance.Parse and pushes
// when the rules actually changed. A bad publish keeps the last good snapshot
// and logs.
type NacosSource struct {
	cli config_client.IConfigClient
	src governanceSource
	doc string // latest document bytes (for dedupe before parsing)

	// governance.PushSource is embedded so this transport inherits the [Source]
	// read and subscribe pair (the nacos listener feeds it through apply).
	// Embedding also promotes Push — the INTERNAL commit path that skips apply's
	// parse and change-dedup guards — so callers must never call Push directly.
	*governance.PushSource
}

// NewNacosSource builds the client from the bound config and seeds the
// snapshot from the dataId's current content.
func NewNacosSource(c governanceNacosConfig) (*NacosSource, error) {
	host, port, err := netutil.SplitHostPort(c.Server)
	if err != nil {
		return nil, err
	}
	cli, err := clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig: constant.NewClientConfig(
			constant.WithNamespaceId(c.Namespace),
			constant.WithTimeoutMs(uint64(c.DialTimeout.Milliseconds())),
			constant.WithUsername(c.Username),
			constant.WithPassword(c.Password),
			constant.WithNotLoadCacheAtStart(true),
		),
		ServerConfigs: []constant.ServerConfig{*constant.NewServerConfig(host, port)},
	})
	if err != nil {
		return nil, errutil.Explain(err, "governance nacos source: create client for %s failed", c.Server)
	}
	return newNacosSource(cli, governanceSource{
		dataID: c.DataID,
		group:  c.Group,
		format: sourceFormat(c.Format, c.DataID),
	})
}

// newNacosSource seeds the snapshot from the dataId's current content on an
// existing client.
func newNacosSource(cli config_client.IConfigClient, src governanceSource) (*NacosSource, error) {
	content, err := cli.GetConfig(vo.ConfigParam{DataId: src.dataID, Group: src.group})
	if err != nil {
		return nil, errutil.Explain(err, "governance nacos source: get %s/%s failed", src.group, src.dataID)
	}
	cfg, err := governance.Parse(src.dataID, []byte(content), src.format)
	if err != nil {
		return nil, err
	}
	return &NacosSource{cli: cli, src: src, doc: content, PushSource: governance.NewPushSource(cfg)}, nil
}

// Init installs the change listener (the gs bean lifecycle hook). The listener
// ctx carries the group/dataId fields once, so every delivery apply handles
// logs with them.
func (s *NacosSource) Init() error {
	ctx := log.RootFields(
		log.String("source", "nacos"),
		log.String("group", s.src.group),
		log.String("data_id", s.src.dataID),
	)
	return s.cli.ListenConfig(vo.ConfigParam{
		DataId: s.src.dataID,
		Group:  s.src.group,
		OnChange: func(namespace, group, dataId, data string) {
			if data != s.doc { // nacos may re-push on reconnect
				s.apply(ctx, data)
			}
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

// apply parses one delivered document and, when the rules actually changed,
// pushes it. The caller drops byte-equal re-deliveries; bad values push
// nothing. The listener ctx already carries the group/dataId fields; the
// delivery only mints its trace_id on top — one delivery is one path, and it
// continues into the center that applies it, so the trace_id is the one both
// lines carry.
func (s *NacosSource) apply(ctx context.Context, data string) {
	ctx = log.WithFields(ctx, log.String("trace_id", randutil.Hex(16)))
	cfg, err := governance.Parse(s.src.dataID, []byte(data), s.src.format)
	if err != nil {
		log.Error(ctx, starterTag, err, "governance nacos source published an invalid document (keeping last good config)")
		return
	}
	s.doc = data
	if reflect.DeepEqual(s.Snapshot(), cfg) {
		return
	}
	s.Push(ctx, cfg)
}
