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

package StarterOutboxGorm

import (
	"time"

	"go-spring.org/cloud/experimental/outbox"
)

// Config binds ${spring.outbox.<name>} — one relay instance per entry. Each
// relay drains one gorm database's outbox_message table to one messaging
// driver. Fields mirror [outbox.Config]; the zero-value relay defaults apply
// (1s poll / batch 100 / 8 attempts / backoff 1s doubling capped 1m / ".dlq").
type Config struct {
	// DB names the *gorm.DB bean backing the outbox table. Empty autowires
	// the single *gorm.DB bean (whatever gorm driver starter provided it).
	DB string `value:"${db:=}"`

	// Driver names the messaging.Driver bean to autowire as the delivery side —
	// the bean a broker starter exports over its configured client (kafka, nats,
	// ...) or one the app provides for its own broker. Empty autowires the single
	// messaging.Driver bean, mirroring DB. Only required when more than one
	// messaging.Driver bean exists.
	Driver string `value:"${driver:=}"`

	// AutoMigrate creates the outbox_message table at startup. Default off:
	// applications that manage schema with a migration tool create it from
	// the documented DDL instead.
	AutoMigrate bool `value:"${auto-migrate:=false}"`

	// PollInterval is the wait between polls when the table is drained.
	PollInterval time.Duration `value:"${poll-interval:=1s}"`

	// BatchSize caps the records fetched per poll.
	BatchSize int `value:"${batch-size:=100}"`

	// MaxAttempts is the total delivery attempts before dead-lettering.
	MaxAttempts int `value:"${max-attempts:=8}"`

	// BackoffBase is the retry wait after the first failure; doubled up to
	// BackoffMax on each further failure.
	BackoffBase time.Duration `value:"${backoff-base:=1s}"`

	// BackoffMax caps one retry wait.
	BackoffMax time.Duration `value:"${backoff-max:=1m}"`

	// DLQSuffix derives the dead-letter destination from the record's own
	// destination ("orders" → "orders.dlq"). Empty disables the DLQ copy:
	// exhausted records go straight to dead status.
	DLQSuffix string `value:"${dlq-suffix:=.dlq}"`
}

// relayConfig projects Config onto the relay's knobs.
func (c Config) relayConfig() outbox.Config {
	return outbox.Config{
		PollInterval: c.PollInterval,
		BatchSize:    c.BatchSize,
		MaxAttempts:  c.MaxAttempts,
		BackoffBase:  c.BackoffBase,
		BackoffMax:   c.BackoffMax,
		DLQSuffix:    c.DLQSuffix,
	}
}
