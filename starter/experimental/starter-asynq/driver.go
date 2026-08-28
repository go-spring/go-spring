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

// driver.go is the "construction seam" of this starter: the Driver interface
// + registry + DefaultDriver, which builds the shared RedisConnOpt from
// Config. It mirrors the other client starters' driver.go.
package StarterAsynq

import (
	"context"
	"crypto/tls"

	"github.com/hibiken/asynq"
	"go-spring.org/cloud/tlsconf"
	"go-spring.org/stdlib/errutil"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Driver builds the Redis connection options for one instance. It is the
// single extension point for customizing the Redis dial (e.g. a sentinel
// topology the default URI form cannot express).
type Driver interface {
	RedisConnOpt(ctx context.Context, c Config) (asynq.RedisConnOpt, error)
}

// RegisterDriver registers an asynq driver with the given name.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("asynq driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// lookupDriver resolves the configured driver name against the registry,
// defaulting to "DefaultDriver" when unset or empty. An unknown name is a
// clear error rather than a silent fallback, so a registration typo fails
// at startup instead of quietly ignoring the custom driver.
func lookupDriver(name string) (Driver, error) {
	if name == "" {
		name = "DefaultDriver"
	}
	d, ok := driverRegistry[name]
	if !ok {
		return nil, errutil.Explain(nil, "asynq driver not found: %s", name)
	}
	return d, nil
}

// DefaultDriver builds the standard host:port RedisConnOpt.
type DefaultDriver struct{}

// RedisConnOpt builds the Redis connection options from Config. TLS is
// enabled only when requested (tlsconf shared block); asynq's URI form
// otherwise matches the plain host:port dial.
func (DefaultDriver) RedisConnOpt(ctx context.Context, c Config) (asynq.RedisConnOpt, error) {
	if c.TLS.Enabled {
		cfg, err := c.TLS.Build()
		if err != nil {
			return nil, errutil.Explain(err, "asynq: build tls config")
		}
		cfg.ServerName = c.TLS.ServerName
		return &asynq.RedisClientOpt{
			Addr:     c.Addr,
			Username: c.Username,
			Password: c.Password,
			DB:       c.DB,
			TLSConfig: &tls.Config{
				ServerName:         cfg.ServerName,
				InsecureSkipVerify: c.TLS.InsecureSkipVerify,
				RootCAs:            cfg.RootCAs,
				Certificates:       cfg.Certificates,
			},
		}, nil
	}
	return asynq.RedisClientOpt{
		Addr:     c.Addr,
		Username: c.Username,
		Password: c.Password,
		DB:       c.DB,
	}, nil
}

// ensure tlsconf stays referenced for the doc comment.
var _ = tlsconf.TLSConfig{}
