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

package StarterTdengine

import (
	"context"
	"strings"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	health2 "go-spring.org/starter-tdengine/health"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple TDengine clients as a group, one per entry under
	// "${spring.tdengine}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration
	// to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.tdengine"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.tdengine}", func(name string, c Config) error {
			// The wrapper bean owns the resilience executor, so Init arms it
			// (InitMethod) and Destroy tears it down.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(w *Client) health.Indicator {
				return health2.NewClientHealth(name, w.DB)
			}, gs.TagArg(name)).Name("tdengine:" + name).Export(gs.As[health.Indicator]()).Caller(1)
			return nil
		})
	})
}

// newClient creates a new TDengine client based on the provided
// configuration. The server is pinged once at startup so that
// misconfiguration or an unreachable taosAdapter fails fast rather than on
// first use.
func newClient(ctx *gs.ContextProvider, c Config) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating tdengine client, dsn-addr=%s", dsnAddr(c.DSN))

	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx.Context, log.TagAppDef, "tdengine driver not found: %s", c.Driver)
		return nil, errutil.Explain(nil, "tdengine driver not found: %s", c.Driver)
	}
	cl, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		return nil, err
	}
	pctx, cancel := context.WithTimeout(ctx.Context, 10*time.Second)
	defer cancel()
	if err = cl.PingContext(pctx); err != nil {
		_ = cl.Close()
		return nil, errutil.Explain(err, "failed to reach tdengine at %s", dsnAddr(c.DSN))
	}
	return cl, nil
}

// HealthCheck reports whether the TDengine instance is reachable. It is a
// thin readiness probe suitable for wiring into a health endpoint.
func HealthCheck(ctx context.Context, client *Client) error {
	return client.PingContext(ctx)
}

// dsnAddr extracts a display-safe address from the unified DSN, e.g.
// "root:taosdata@ws(127.0.0.1:6041)/power" -> "127.0.0.1:6041".
func dsnAddr(dsn string) string {
	if i := strings.Index(dsn, "("); i >= 0 {
		if j := strings.Index(dsn[i:], ")"); j > 0 {
			return dsn[i+1 : i+j]
		}
	}
	return dsn
}
