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

package main

import (
	"context"

	klog "github.com/go-kratos/kratos/v2/log"
	"go-spring.org/log"
)

// gsBridgeLogger implements kratos' log.Logger by forwarding every framework log
// line into go-spring's log module. This is the pilot of a wider goal: let each
// contrib framework's internal logs flow through go-spring's log so users only
// configure one logging pipeline (here the root FileLogger in conf/app.properties,
// which ships to Loki). kratos' transport start/stop, etcd registration and
// middleware errors thus land in provider.log as JSON, next to the business logs.
//
// Trade-off: kratos' Log signature carries no context.Context, so trace-id
// propagation via go-spring's FieldsFromContext hook is not available on this
// path, and the caller (file:line) recorded by go-spring points into this bridge
// rather than the real emit site. We deliberately keep the structured fields and
// accept the caller imprecision (see the log-bridge design discussion).
type gsBridgeLogger struct {
	tag *log.Tag
}

// newGSBridgeLogger builds the bridge, tagging every forwarded line as an RPC log
// under "rpc.kratos" so it can be filtered or routed to a dedicated logger later
// without touching the framework wiring.
func newGSBridgeLogger() klog.Logger {
	return &gsBridgeLogger{tag: log.RegisterRPCTag("kratos", "")}
}

// toGSLevel maps kratos log levels onto go-spring's levels. kratos has no Trace
// or Panic level, so the mapping is a straight one-to-one over the shared five.
func toGSLevel(level klog.Level) log.Level {
	switch level {
	case klog.LevelDebug:
		return log.DebugLevel
	case klog.LevelInfo:
		return log.InfoLevel
	case klog.LevelWarn:
		return log.WarnLevel
	case klog.LevelError:
		return log.ErrorLevel
	case klog.LevelFatal:
		return log.FatalLevel
	default:
		return log.InfoLevel
	}
}

// Log converts kratos' flat keyvals into go-spring fields and records the event.
// kratos passes keyvals in key/value pairs; by convention the message lives under
// the "msg" key and the level under "level" (injected by kratos' own level
// valuer). We lift "msg" into the event message, drop the redundant "level" (the
// event already carries it), and forward everything else as structured fields.
func (l *gsBridgeLogger) Log(level klog.Level, keyvals ...any) error {
	// kratos guarantees pairs, but guard against a stray odd count like kratos'
	// own Helper does, so a malformed call still logs rather than panics.
	if len(keyvals)%2 != 0 {
		keyvals = append(keyvals, "KEYVALS UNPAIRED")
	}

	fields := make([]log.Field, 0, len(keyvals)/2)
	for i := 0; i < len(keyvals); i += 2 {
		key, _ := keyvals[i].(string)
		val := keyvals[i+1]
		switch key {
		case "msg":
			fields = append(fields, log.Msgf("%v", val))
		case klog.LevelKey:
			// Already represented by the event's own level; skip to avoid a
			// duplicate "level" field in the output.
		default:
			fields = append(fields, log.Any(key, val))
		}
	}

	// No context is available on kratos' Log signature (see type doc). skip=2
	// steps out of this method and go-spring's record(), but the caller is
	// inherently unreliable across the framework's logging layers.
	log.Record(context.Background(), toGSLevel(level), l.tag, 2, fields...)
	return nil
}
