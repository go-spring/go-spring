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

package StarterGovernanceNacos

import (
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/governance"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/netutil"
)

func init() {
	gs.Module(gs.OnProperty("spring.governance.source.nacos"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c governanceNacosConfig
		if err := conf.Bind(p, &c, "${spring.governance.source.nacos:=}"); err != nil {
			return err
		}
		src := governanceSource{
			dataID: c.DataID,
			group:  c.Group,
			format: sourceFormat(c.Format, c.DataID),
		}

		r.Provide(func() (*NacosSource, error) {
			host, port, err := netutil.SplitHostPort(c.Server)
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
