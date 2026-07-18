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

package StarterGoRedis

import (
	"context"
	"runtime"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/health"
)

func init() {
	// Register Redis clients as a group, one per entry under "${spring.go-redis}".
	//
	// Unlike a plain gs.Group, the bean type is chosen per entry from its Mode:
	// single/sentinel entries register a *redis.Client, cluster entries register
	// a *redis.ClusterClient (go-redis returns distinct types for the two). A
	// single Group cannot mix return types, so we bind the map ourselves and
	// dispatch. Switching a client to cluster is then an in-config change plus
	// swapping the injected type from *redis.Client to *redis.ClusterClient.
	_, file, line, _ := runtime.Caller(0)
	gs.Module(gs.OnProperty("spring.go-redis"), func(r gs.BeanProvider, p flatten.Storage) error {
		var m map[string]Config
		if err := conf.Bind(p, &m, "${spring.go-redis}"); err != nil {
			return err
		}
		for name, c := range m {
			switch c.Mode {
			case "", "single", "sentinel":
				b := r.Provide(newClient, gs.ValueArg(c)).Name(name).Destroy(destroyClient)
				b.SetFileLine(file, line)
				// Contribute a health indicator for this instance, injecting the
				// client just registered above by name.
				h := r.Provide(newClientHealth, gs.ValueArg(name), gs.TagArg(name)).Export(gs.As[health.Indicator]())
				h.SetFileLine(file, line)
			case "cluster":
				b := r.Provide(newClusterClient, gs.ValueArg(c)).Name(name).Destroy(destroyClusterClient)
				b.SetFileLine(file, line)
				h := r.Provide(newClusterHealth, gs.ValueArg(name), gs.TagArg(name)).Export(gs.As[health.Indicator]())
				h.SetFileLine(file, line)
			default:
				return errutil.Explain(nil, "redis: invalid mode %q for instance %q (want single/sentinel/cluster)", c.Mode, name)
			}
		}
		return nil
	})
}

// newClient creates a single or sentinel Redis client (*redis.Client), bridged
// into go-spring's unified observability. The redisotel hooks emit client spans
// and connection-pool metrics through the OTel globals that starter-otel
// installs; when starter-otel is absent those globals are no-ops, so this stays
// a zero-config opt-in that needs no per-component adaptation.
func newClient(c Config) (*redis.Client, error) {
	if err := validateConfig(c); err != nil {
		return nil, err
	}
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "redis driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(c)
	if err != nil {
		return nil, err
	}
	if err := instrument(client); err != nil {
		_ = client.Close()
		return nil, err
	}
	if err := failFastPing(c, client); err != nil {
		_ = destroyClient(client)
		return nil, err
	}
	if err := applyResilience(c, client); err != nil {
		_ = destroyClient(client)
		return nil, err
	}
	return client, nil
}

// newClusterClient creates a cluster Redis client (*redis.ClusterClient). The
// driver must implement ClusterDriver; the redisotel hooks attach per-node via
// ClusterClient.OnNewNode, so tracing/metrics cover every node discovered.
func newClusterClient(c Config) (*redis.ClusterClient, error) {
	if err := validateConfig(c); err != nil {
		return nil, err
	}
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "redis driver not found: %s", c.Driver)
	}
	cd, ok := d.(ClusterDriver)
	if !ok {
		return nil, errutil.Explain(nil, "redis driver %q does not support cluster mode", c.Driver)
	}
	client, err := cd.CreateClusterClient(c)
	if err != nil {
		return nil, err
	}
	if err := instrument(client); err != nil {
		_ = client.Close()
		return nil, err
	}
	if err := failFastPing(c, client); err != nil {
		_ = client.Close()
		return nil, err
	}
	if err := applyResilience(c, client); err != nil {
		_ = destroyClusterClient(client)
		return nil, err
	}
	return client, nil
}

// validateConfig checks the per-mode required fields, and rejects combining
// service discovery with sentinel/cluster (which self-discover their nodes).
func validateConfig(c Config) error {
	switch c.Mode {
	case "", "single":
		if c.Addr == "" && c.ServiceName == "" {
			return errutil.Explain(nil, "redis: one of addr or service-name must be set")
		}
	case "sentinel":
		if c.ServiceName != "" {
			return errutil.Explain(nil, "redis: service-name is not supported in sentinel mode")
		}
		if c.MasterName == "" || len(c.SentinelAddrs) == 0 {
			return errutil.Explain(nil, "redis: master-name and sentinel-addrs are required in sentinel mode")
		}
	case "cluster":
		if c.ServiceName != "" {
			return errutil.Explain(nil, "redis: service-name is not supported in cluster mode")
		}
		if len(c.Addrs) == 0 {
			return errutil.Explain(nil, "redis: addrs is required in cluster mode")
		}
	}
	return nil
}

// instrument attaches redisotel tracing and metrics. It accepts any topology via
// redis.UniversalClient (*redis.Client and *redis.ClusterClient both satisfy it).
func instrument(client redis.UniversalClient) error {
	if err := redisotel.InstrumentTracing(client); err != nil {
		return err
	}
	return redisotel.InstrumentMetrics(client)
}

// failFastPing verifies the connection is usable at startup so a misconfigured
// address or unreachable server surfaces during boot rather than on the first
// request. It applies to all three topologies. The DialTimeout bounds the probe.
func failFastPing(c Config, client redis.UniversalClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout(c))
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return errutil.Explain(err, "redis: startup ping failed")
	}
	return nil
}

// pingTimeout picks a bound for the startup ping: the configured DialTimeout
// when set, otherwise a conservative default.
func pingTimeout(c Config) time.Duration {
	if c.DialTimeout > 0 {
		return c.DialTimeout
	}
	return 5 * time.Second
}

// destroyClient closes a single/sentinel client and stops any discovery watch
// behind it.
func destroyClient(client *redis.Client) error {
	closeResilience(client)
	if v, ok := liveDialers.LoadAndDelete(client); ok {
		_ = v.(*discovery.LiveDialer).Stop()
	}
	return client.Close()
}

// destroyClusterClient closes a cluster client. Cluster mode never uses a
// LiveDialer, so there is no discovery watch to stop.
func destroyClusterClient(client *redis.ClusterClient) error {
	closeResilience(client)
	return client.Close()
}
