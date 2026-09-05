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

// observe.go is the per-message access log of this starter. It is log-only on
// purpose: kotel (wired in driver.go) already provides producer/consumer spans
// and client metrics, so this layer only fills the access-log gap.
package StarterKafka

import (
	"context"
	"go-spring.org/stdlib/strutil"
	"time"

	"go-spring.org/log"
)

// accessTag is the static log tag for the kafka access log.
var accessTag = log.RegisterAppTag("kafka", "access")

// accessRecord is one in-flight access-log record, opened by startAccess and
// closed by exactly one End, which emits the log line carrying the measured
// duration.
type accessRecord struct {
	ctx   context.Context
	op    string
	arg   string
	start time.Time
}

// startAccess opens an access-log record for op (e.g. "publish", "consume");
// arg is the destination topic, captured in the log when non-empty.
func startAccess(ctx context.Context, op, arg string) *accessRecord {
	return &accessRecord{ctx: ctx, op: op, arg: arg, start: time.Now()}
}

// End emits the access record. The log level carries the outcome: an error at
// Warn, a success with a destination at Debug (the per-message record is
// frequent and uninteresting until it fails), a success without a destination
// at Info.
func (s *accessRecord) End(err error) {
	fields := func() []log.Field {
		f := []log.Field{
			log.String("operation", s.op),
			log.Float("duration_ms", float64(time.Since(s.start).Nanoseconds())/1e6),
		}
		if s.arg != "" {
			f = append(f, log.String("destination", strutil.Truncate(s.arg, 512)))
		}
		return f
	}
	if err != nil {
		log.Warn(s.ctx, accessTag, append(fields(), log.Any("error", err))...)
		return
	}
	if s.arg != "" {
		log.Debug(s.ctx, accessTag, fields)
		return
	}
	log.Info(s.ctx, accessTag, fields()...)
}
