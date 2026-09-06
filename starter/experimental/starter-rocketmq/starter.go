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

// starter.go is the gs registration + glue concept of this starter: it declares
// the infra log tag and registers the per-instance RocketMQ client group under
// "${spring.rocketmq}", wiring each Config entry to newClient (the dispatch +
// probe + resilience wiring) and the Client wrapper's Close (the lifecycle in
// client.go).
package StarterRocketmq

import (
	"strings"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {

	// Register multiple RocketMQ clients as a group.
	// Each instance is created according to the configuration in "${spring.rocketmq}".
	// This allows defining multiple RocketMQ clients dynamically.
	gs.Module(gs.OnProperty("spring.rocketmq"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.rocketmq}", func(name string, c Config) error {
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("?")),
			).Name(name).Destroy((*Client).Close).Caller(1)
			return nil
		})
	})
}

// newClient creates a RocketMQ client by dispatching to the (optional) Driver
// bean, which owns full client assembly (name server resolution, credentials,
// the rlog bridge). After the client is built it is probed (when FailFast is
// enabled) so a wrong name server list fails fast at startup instead of
// surfacing on the first produce/consume, then the resilience executor is
// attached.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating rocketmq client, name-servers=%v fail-fast=%v", c.NameServers, c.FailFast)

	if (c.AccessKey == "") != (c.SecretKey == "") {
		return nil, errutil.Explain(nil, "rocketmq access-key and secret-key must be set together (client %s)", name)
	}

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		return nil, err
	}

	if c.FailFast {
		if err = probeNameServer(c.NameServers); err != nil {
			log.Errorf(ctx.Context, log.TagAppDef, "rocketmq: fail-fast probe failed on %v: %v", c.NameServers, err)
			return nil, errutil.Explain(err, "rocketmq name server probe failed on %v", c.NameServers)
		}
	}
	if err := applyResilience(c, cl, resilience.ResourceLabel("rocketmq", strings.Join(c.NameServers, ","))); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "rocketmq: resilience setup failed: %v", err)
		return nil, err
	}
	log.Infof(ctx.Context, log.TagAppDef, "rocketmq client initialized, name-servers=%v", c.NameServers)
	return cl, nil
}
