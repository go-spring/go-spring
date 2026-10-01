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

package StarterGovernanceEtcd

import (
	"go-spring.org/cloud/governance"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {
	gs.Module(gs.OnProperty("spring.governance.source.etcd"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c governanceEtcdConfig
		if err := conf.Bind(p, &c, "${spring.governance.source.etcd:=}"); err != nil {
			return err
		}
		format := sourceFormat(c.Format, c.Key)

		r.Provide(func() (*EtcdSource, error) {
			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   []string{c.Endpoint},
				Username:    c.Username,
				Password:    c.Password,
				DialTimeout: dialTimeout,
			})
			if err != nil {
				return nil, errutil.Explain(err, "governance etcd source: create client for %s failed", c.Endpoint)
			}
			return NewEtcdSource(cli, c.Key, format)
		}).
			Init((*EtcdSource).Init).Destroy((*EtcdSource).Close).
			Export(gs.As[governance.Source]()).Caller(1)
		return nil
	})
}
