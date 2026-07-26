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

package StarterMongoDB

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/experimental/cloud/discovery"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// liveDialers tracks the discovery-backed dialer behind each client, so
// destroyClient can stop the background watch on teardown.
var liveDialers sync.Map // *mongo.Client -> *discovery.LiveDialer

var starterTag = log.RegisterInfraTag("mongodb", "")

func init() {

	// Register multiple MongoDB clients as a group.
	// Each instance is created according to the configuration in "${spring.mongodb}".
	// This allows defining multiple MongoDB clients dynamically.
	gs.Group("${spring.mongodb}", newClient, destroyClient)
}

// newClient creates a new MongoDB client based on the provided configuration,
// bridged into go-spring's unified observability via a command monitor (see
// observability.go). After the client is built it is pinged so that
// misconfiguration or an unreachable server fails fast at startup rather than
// on first use.
//
// When c.ServiceName is set, the address is resolved through the registered
// discovery backend (c.Discovery): a LiveDialer is injected as the client's
// ContextDialer, so each new connection dials a currently-live instance and
// address changes take effect without rebuilding the client. When c.ServiceName
// is empty this dials the URI hosts directly, unchanged from before.
func newClient(cp *gs.ContextProvider, c Config) (*mongo.Client, error) {
	ctx := cp.Context
	log.Debugf(ctx, starterTag, "creating mongodb client, uri=%s service-name=%s", c.URI, c.ServiceName)

	opts := options.Client().ApplyURI(c.URI)
	opts.SetMonitor(newCommandMonitor())
	if c.ConnectTimeout > 0 {
		opts.SetConnectTimeout(c.ConnectTimeout)
	}
	if c.ServerSelectionTimeout > 0 {
		opts.SetServerSelectionTimeout(c.ServerSelectionTimeout)
	}
	if c.MaxPoolSize > 0 {
		opts.SetMaxPoolSize(c.MaxPoolSize)
	}
	opts.SetMinPoolSize(c.MinPoolSize)
	if c.MaxConnIdleTime > 0 {
		opts.SetMaxConnIdleTime(c.MaxConnIdleTime)
	}
	if c.Username != "" {
		opts.SetAuth(options.Credential{
			Username:      c.Username,
			Password:      c.Password,
			AuthSource:    c.AuthSource,
			AuthMechanism: c.AuthMechanism,
		})
	}
	tlsCfg, err := c.TLS.Build()
	if err != nil {
		log.Errorf(ctx, starterTag, "mongodb: build TLS failed: %v", err)
		return nil, errutil.Explain(err, "mongodb: build TLS")
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	var ld *discovery.LiveDialer
	if c.ServiceName != "" {
		d, err := discovery.MustGet(c.Discovery)
		if err != nil {
			log.Errorf(ctx, starterTag, "mongodb: get discovery backend failed: %v", err)
			return nil, err
		}
		ld, err = discovery.NewLiveDialer(ctx, d, c.ServiceName)
		if err != nil {
			log.Errorf(ctx, starterTag, "mongodb: create live dialer for %s failed: %v", c.ServiceName, err)
			return nil, err
		}
		// The LiveDialer ignores the dialed address and picks a live endpoint;
		// its DialContext matches the options.ContextDialer interface directly.
		opts.SetDialer(ld)
	}

	client, err := mongo.Connect(opts)
	if err != nil {
		log.Errorf(ctx, starterTag, "mongodb: connect failed: %v", err)
		if ld != nil {
			_ = ld.Stop()
		}
		return nil, fmt.Errorf("mongodb: create client: %w", err)
	}

	// Fail fast: verify the server is reachable before handing out the client.
	pingCtx, cancel := pingContext(ctx, c.ConnectTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		log.Errorf(ctx, starterTag, "mongodb: ping failed uri=%s: %v", c.URI, err)
		_ = client.Disconnect(context.Background())
		if ld != nil {
			_ = ld.Stop()
		}
		return nil, fmt.Errorf("mongodb: ping %s: %w", c.URI, err)
	}
	if ld != nil {
		liveDialers.Store(client, ld)
	}
	log.Infof(ctx, starterTag, "mongodb client initialized, uri=%s", c.URI)
	return client, nil
}

// HealthCheck reports whether the MongoDB client can reach the server. It is a
// thin readiness probe suitable for wiring into a health endpoint.
func HealthCheck(ctx context.Context, client *mongo.Client) error {
	return client.Ping(ctx, nil)
}

// pingContext derives a context for the startup ping, bounded by the connect
// timeout when set so the probe cannot hang indefinitely.
func pingContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

// destroyClient disconnects the MongoDB client and stops any discovery watch
// behind it.
func destroyClient(client *mongo.Client) error {
	if v, ok := liveDialers.LoadAndDelete(client); ok {
		_ = v.(*discovery.LiveDialer).Stop()
		log.Debugf(context.Background(), starterTag, "mongodb client destroyed, discovery dialer stopped")
	}
	return client.Disconnect(context.Background())
}
