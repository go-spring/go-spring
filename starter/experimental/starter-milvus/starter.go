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

package StarterMilvus

import (
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	health2 "go-spring.org/starter-milvus/health"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	gs.Module(gs.OnProperty("spring.milvus"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.milvus}", func(name string, c Config) error {
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)

			r.Provide(func(w *Client) health.Indicator {
				return health2.NewClientHealth(name, w)
			}, gs.TagArg(name)).Name("milvus:" + name).Export(gs.As[health.Indicator]()).Caller(1)
			return nil
		})
	})
}

// ensure errutil stays referenced in this file's error path (used by future
// driver dispatch if added).
var _ = errutil.Explain
