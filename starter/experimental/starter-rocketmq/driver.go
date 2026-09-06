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
// interface + the bundled DefaultDriver, which owns full client assembly (the
// rlog bridge into go-spring's log, name server parsing, credential building)
// and hands back the Client wrapper. It mirrors starter-pulsar's driver.go.
package StarterRocketmq

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/rlog"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a RocketMQ client (the starter's
// Client wrapper). It is an OPTIONAL CONTAINER BEAN: a company or umbrella
// starter may provide its own Driver bean (its constructor returns
// StarterRocketmq.Driver); when none is present, starter-rocketmq falls back
// to the bundled [DefaultDriver] inside client assembly. A custom driver is a
// bean, so it may inject the configuration/beans it needs — e.g. company
// config bound from a properties file at wiring time.
//
// At most one Driver bean is expected per process; every client under
// ${spring.rocketmq} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new RocketMQ Client from the provided configuration.
// It owns full client assembly — the rlog bridge into go-spring's log, the
// name server list and the credential builder stored on the wrapper — but not
// the startup name server probe (FailFast) or the resilience wiring, which
// are the starter's lifecycle concerns (see newClient in starter.go).
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*Client, error) {
	// Bridge rocketmq-client-go's internal logs into go-spring's log so
	// connection and rebalance events show up alongside application logs.
	// rlog.SetLogger is process-global, so install the bridge exactly once.
	installLogBridge()

	ns := primitive.NamesrvAddr(c.NameServers)
	cl := &Client{
		nameServers: ns,
		cfg:         c,
	}
	return cl, nil
}

// -----------------------------------------------------------------------------
// Log bridge
// -----------------------------------------------------------------------------

// bridgeLogger adapts rocketmq-client-go's rlog.Logger into go-spring's log.
// Fields are folded into the message so the bridge stays independent of
// go-spring's field API.
type bridgeLogger struct{}

var installLogBridgeOnce sync.Once

// installLogBridge installs the rlog bridge exactly once per process.
func installLogBridge() {
	installLogBridgeOnce.Do(func() { rlog.SetLogger(bridgeLogger{}) })
}

func (bridgeLogger) Debug(msg string, fields map[string]interface{}) {
	emit(log.DebugLevel, msg, fields)
}

func (bridgeLogger) Info(msg string, fields map[string]interface{}) {
	emit(log.InfoLevel, msg, fields)
}

func (bridgeLogger) Warning(msg string, fields map[string]interface{}) {
	emit(log.WarnLevel, msg, fields)
}

func (bridgeLogger) Error(msg string, fields map[string]interface{}) {
	emit(log.ErrorLevel, msg, fields)
}

func (bridgeLogger) Fatal(msg string, fields map[string]interface{}) {
	emit(log.ErrorLevel, msg, fields)
}

// Level and OutputPath are no-ops: the level is governed by go-spring's log
// configuration and output goes through the same sink.
func (bridgeLogger) Level(string)            {}
func (bridgeLogger) OutputPath(string) error { return nil }

// emit forwards a rocketmq log line to go-spring's log at the mapped level.
func emit(level log.Level, msg string, fields map[string]interface{}) {
	ctx := context.Background()
	line := msg
	if len(fields) > 0 {
		line = fmt.Sprintf("%s %v", msg, fields)
	}
	switch level {
	case log.ErrorLevel:
		log.Errorf(ctx, log.TagAppDef, "rocketmq: %s", line)
	case log.WarnLevel:
		log.Warnf(ctx, log.TagAppDef, "rocketmq: %s", line)
	case log.DebugLevel:
		log.Debugf(ctx, log.TagAppDef, "rocketmq: %s", line)
	default:
		log.Infof(ctx, log.TagAppDef, "rocketmq: %s", line)
	}
}

// probeNameServer dials the first reachable address in the name server list
// with a short timeout. RocketMQ's remoting layer connects lazily — a wrong
// address list would otherwise surface only on the first produce/consume — so
// the fail-fast path verifies TCP reachability up front.
func probeNameServer(addrs []string) error {
	var lastErr error
	for _, addr := range addrs {
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errutil.Explain(nil, "empty name server list")
	}
	return lastErr
}
