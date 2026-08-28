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

// driver.go is the "construction seam" concept of this starter: the Driver
// interface + registry + DefaultDriver, which owns full client assembly
// (ClientOptions, authentication — mTLS / token — TLS, the native Prometheus
// metrics registry, the go-spring log bridge, and pulsar.NewClient). It mirrors
// starter-kafka's driver.go.
package StarterPulsar

import (
	"context"
	"fmt"
	"net/http"

	"github.com/apache/pulsar-client-go/pulsar"
	plog "github.com/apache/pulsar-client-go/pulsar/log"
	"github.com/prometheus/client_golang/prometheus"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Driver interface defines how to create a Pulsar client (a pulsar.Client). It
// is the single extension point for customizing client assembly: a company (or
// the bundled DefaultDriver) implements it once and registers via
// RegisterDriver; callers select one through Config.Driver, which defaults to
// "DefaultDriver".
type Driver interface {
	CreateClient(ctx context.Context, c Config) (pulsar.Client, error)
}

// RegisterDriver registers a Pulsar driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("pulsar driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new pulsar.Client from the provided configuration. It
// owns full client assembly — ClientOptions, authentication (mTLS / token), TLS,
// the native Prometheus metrics registry and its /metrics server, the log
// bridge, and the pulsar.NewClient call — but not the startup broker probe
// (FailFast) or the resilience wiring, which are the starter's lifecycle
// concerns (see newClient in starter.go).
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (pulsar.Client, error) {
	opts := pulsar.ClientOptions{
		URL:                        c.URL,
		OperationTimeout:           c.OperationTimeout,
		ConnectionTimeout:          c.ConnectionTimeout,
		TLSTrustCertsFilePath:      c.TLSTrustCertsFilePath,
		TLSCertificateFile:         c.TLSCertFile,
		TLSKeyFilePath:             c.TLSKeyFile,
		TLSAllowInsecureConnection: c.TLSAllowInsecure,
		TLSValidateHostname:        c.TLSValidateHostname,
		Logger:                     newLogger(),
	}

	// Authentication: prefer mTLS (cert+key) if both are set, else token.
	switch {
	case c.TLSCertFile != "" && c.TLSKeyFile != "":
		opts.Authentication = pulsar.NewAuthenticationTLS(c.TLSCertFile, c.TLSKeyFile)
	case c.Token != "" && c.TokenFromFile:
		opts.Authentication = pulsar.NewAuthenticationTokenFromFile(c.Token)
	case c.Token != "":
		opts.Authentication = pulsar.NewAuthenticationToken(c.Token)
	}

	// Expose the client's native Prometheus metrics via a dedicated registry and
	// /metrics endpoint when enabled. The server is remembered so destroyClient
	// can shut it down; pulsar defaults MetricsRegisterer to the process-wide
	// DefaultRegisterer, which we deliberately avoid to keep instances isolated.
	var srv *http.Server
	if c.Metrics.Enabled {
		var reg prometheus.Registerer
		reg, srv = newMetricsServer(c.Metrics)
		opts.MetricsRegisterer = reg
	}

	cl, err := pulsar.NewClient(opts)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "pulsar: create client failed: %v", err)
		if srv != nil {
			_ = srv.Shutdown(ctx)
		}
		return nil, errutil.Explain(err, "failed to create pulsar client: %s", c.URL)
	}
	if srv != nil {
		metricsServers.Store(cl, srv)
	}
	return cl, nil
}

// -----------------------------------------------------------------------------
// Log bridge
// -----------------------------------------------------------------------------

// logger bridges pulsar-client-go's internal log.Logger into go-spring's log,
// so connection events (broker connects, lookup failures, reconnects) show up
// alongside application logs.
//
// pulsar's log.Logger interface has SubLogger/WithFields/WithField/WithError
// plus Debug/Info/Warn/Error and their *f variants; the returned Entry has
// the same output methods. Fields are folded into the message so the bridge
// stays independent of go-spring's field API.
type logger struct {
	fields plog.Fields
}

func newLogger() plog.Logger { return &logger{} }

// SubLogger returns a logger that inherits and extends the field set.
func (l *logger) SubLogger(fields plog.Fields) plog.Logger {
	return &logger{fields: mergeFields(l.fields, fields)}
}

// WithFields / WithField / WithError return an Entry carrying extra fields.
func (l *logger) WithFields(fields plog.Fields) plog.Entry {
	return &entry{fields: mergeFields(l.fields, fields)}
}

func (l *logger) WithField(name string, value any) plog.Entry {
	return l.WithFields(plog.Fields{name: value})
}

func (l *logger) WithError(err error) plog.Entry {
	return l.WithFields(plog.Fields{"error": err})
}

func (l *logger) Debug(args ...any) { emit(log.DebugLevel, l.fields, fmt.Sprint(args...)) }
func (l *logger) Info(args ...any)  { emit(log.InfoLevel, l.fields, fmt.Sprint(args...)) }
func (l *logger) Warn(args ...any)  { emit(log.WarnLevel, l.fields, fmt.Sprint(args...)) }
func (l *logger) Error(args ...any) { emit(log.ErrorLevel, l.fields, fmt.Sprint(args...)) }

func (l *logger) Debugf(format string, args ...any) {
	emit(log.DebugLevel, l.fields, fmt.Sprintf(format, args...))
}
func (l *logger) Infof(format string, args ...any) {
	emit(log.InfoLevel, l.fields, fmt.Sprintf(format, args...))
}
func (l *logger) Warnf(format string, args ...any) {
	emit(log.WarnLevel, l.fields, fmt.Sprintf(format, args...))
}
func (l *logger) Errorf(format string, args ...any) {
	emit(log.ErrorLevel, l.fields, fmt.Sprintf(format, args...))
}

// entry is pulsar's per-call Entry variant of logger, carrying extra fields.
type entry struct {
	fields plog.Fields
}

func (e *entry) WithFields(fields plog.Fields) plog.Entry {
	return &entry{fields: mergeFields(e.fields, fields)}
}

func (e *entry) WithField(name string, value any) plog.Entry {
	return e.WithFields(plog.Fields{name: value})
}

func (e *entry) Debug(args ...any) { emit(log.DebugLevel, e.fields, fmt.Sprint(args...)) }
func (e *entry) Info(args ...any)  { emit(log.InfoLevel, e.fields, fmt.Sprint(args...)) }
func (e *entry) Warn(args ...any)  { emit(log.WarnLevel, e.fields, fmt.Sprint(args...)) }
func (e *entry) Error(args ...any) { emit(log.ErrorLevel, e.fields, fmt.Sprint(args...)) }

func (e *entry) Debugf(format string, args ...any) {
	emit(log.DebugLevel, e.fields, fmt.Sprintf(format, args...))
}
func (e *entry) Infof(format string, args ...any) {
	emit(log.InfoLevel, e.fields, fmt.Sprintf(format, args...))
}
func (e *entry) Warnf(format string, args ...any) {
	emit(log.WarnLevel, e.fields, fmt.Sprintf(format, args...))
}
func (e *entry) Errorf(format string, args ...any) {
	emit(log.ErrorLevel, e.fields, fmt.Sprintf(format, args...))
}

// mergeFields returns a new map combining base and extra; extra wins on conflict.
func mergeFields(base, extra plog.Fields) plog.Fields {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make(plog.Fields, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// emit forwards a pulsar log line to go-spring's log at the mapped level.
func emit(level log.Level, fields plog.Fields, msg string) {
	ctx := context.Background()
	line := msg
	if len(fields) > 0 {
		line = fmt.Sprintf("%s %v", msg, fields)
	}
	switch level {
	case log.ErrorLevel:
		log.Errorf(ctx, log.TagAppDef, "pulsar: %s", line)
	case log.WarnLevel:
		log.Warnf(ctx, log.TagAppDef, "pulsar: %s", line)
	case log.DebugLevel:
		log.Debugf(ctx, log.TagAppDef, "pulsar: %s", line)
	default:
		log.Infof(ctx, log.TagAppDef, "pulsar: %s", line)
	}
}
