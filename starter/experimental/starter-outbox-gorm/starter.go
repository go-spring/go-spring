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

// Package StarterOutboxGorm contributes a gorm-backed transactional outbox:
// business writes and message publishes commit atomically in one database
// transaction, then a background relay drains the outbox_message table to a
// messaging driver (kafka, nats, ...). Enable one relay instance per entry
// under spring.outbox:
//
//	import _ "go-spring.org/starter-outbox-gorm"
//
//	# the delivery messaging.Driver bean is autowired (a broker starter, e.g.
//	# starter-kafka, exports it over its configured client); name it only when
//	# several exist — spring.outbox.instances.main.driver=kafka
//	# spring.outbox.instances.main.auto-migrate=true
//
// The write side is a plain function, not a bean — it runs inside the
// application's own gorm transaction:
//
//	err := db.Transaction(func(tx *gorm.DB) error {
//	    if err := tx.Create(&order).Error; err != nil { return err }
//	    return StarterOutboxGorm.Publish(tx, "orders", order.ID, payload, nil)
//	})
package StarterOutboxGorm

import (
	"context"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/experimental/outbox"
	"go-spring.org/cloud/messaging"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
	"gorm.io/gorm"
)

func init() {
	// Register one relay instance per entry under "${spring.outbox}". The relay
	// runs as a background loop on the bean's Init/Destroy, not as a gs.Server:
	// it is not an endpoint but a worker, and blocking on it would defeat the
	// readiness signal.
	gs.Module(gs.OnProperty("spring.outbox.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.outbox.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.outbox.instances", "spring.outbox.default")
		return conf.BindEach(p, "${spring.outbox.instances}", func(name string, c Config) error {
			r.Provide(newRelay,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg(c.DB)),
				gs.IndexArg(3, gs.TagArg(c.Driver)),
				gs.IndexArg(4, gs.ValueArg(name)),
			).Name(name).Init((*Relay).Init).Destroy((*Relay).Destroy).
				Export(gs.As[gs.Rooter]()).Caller(1)

			// Health indicator probes the backing database through gorm, unless
			// the user disabled it (health=false).
			if c.Health {
				r.Provide(func(db *gorm.DB) *health.Indicator {
					return NewRelayHealth(name, db)
				}, gs.TagArg(c.DB)).Name("outbox:" + name).Caller(1)
			}
			return nil
		})
	})
}

var starterTag = log.RegisterAppTag("outbox", "")

// Relay is the bean wrapping one [outbox.Relay] loop.
type Relay struct {
	cfg          Config
	instanceName string
	db           *gorm.DB
	drv          messaging.Driver
	relay        *outbox.Relay
	cancel       context.CancelFunc
	done         chan struct{}
}

// newRelay builds the bean from the bound config, the autowired *gorm.DB, the
// autowired messaging.Driver bean and the instance name. The driver is injected
// like any client bean (empty config names the single messaging.Driver bean);
// the container resolves it before Init runs.
func newRelay(_ *gs.ContextProvider, c Config, db *gorm.DB, drv messaging.Driver, name string) (*Relay, error) {
	return &Relay{cfg: c, instanceName: name, db: db, drv: drv}, nil
}

// Init optionally migrates the table, then starts the relay loop in the
// background. The delivery driver was injected at construction; the relay
// publishes through it once the loop polls a pending record.
func (o *Relay) Init() error {
	// The relay's identity rides on a context derived here: every line below
	// carries it without repeating it. The context is minted locally — the
	// bean's own lifecycle entry, not a shared application context.
	octx := log.WithFields(context.Background(), log.String("instance", o.instanceName))

	if o.cfg.AutoMigrate {
		if err := Migrate(o.db); err != nil {
			log.Error(octx, starterTag, err, log.Msg("outbox: auto-migrate failed"))
			return err
		}
	}
	o.relay = outbox.NewRelay(newStore(o.db), o.drv, o.cfg.relayConfig(), logObserver{})
	ctx, cancel := context.WithCancel(context.Background())
	o.cancel, o.done = cancel, make(chan struct{})
	go func() {
		defer close(o.done)
		_ = o.relay.Run(ctx)
		_ = o.relay.Close()
	}()
	driver := o.cfg.Driver
	if driver == "" {
		driver = "(autowired messaging.Driver bean)"
	}
	log.Info(octx, starterTag, log.String("driver", driver), log.Msg("start outbox relay success"))
	return nil
}

// DrainTimeout bounds how long Destroy waits for the loop to exit.
const DrainTimeout = 5 * time.Second

// Destroy stops the loop and waits (bounded) for it to drain.
func (o *Relay) Destroy() error {
	if o.cancel == nil {
		return nil
	}
	// The relay's identity rides on a context derived here, teardown's own
	// scope, so the line below carries it without repeating it.
	octx := log.WithFields(context.Background(), log.String("instance", o.instanceName))
	o.cancel()
	select {
	case <-o.done:
	case <-time.After(DrainTimeout):
		log.Warn(octx, starterTag, log.Msg("outbox relay: drain timed out"))
	}
	return nil
}

// logObserver is the default [outbox.Observer]: retries and dead letters are
// worth a log line; successful publishes are not (the broker-side access log
// already records them).
type logObserver struct{}

// OnPublished implements [outbox.Observer].
func (logObserver) OnPublished(rec *outbox.Record) {}

// OnRetry implements [outbox.Observer].
func (logObserver) OnRetry(rec *outbox.Record, err error, nextRetry time.Time) {
	log.Warn(context.Background(), starterTag,
		log.Int("id", rec.ID),
		log.String("destination", rec.Destination),
		log.Err(err),
		log.String("retry_at", nextRetry.Format(time.RFC3339)),
		log.Msg("outbox: record send failed, will retry"))
}

// OnDead implements [outbox.Observer].
func (logObserver) OnDead(rec *outbox.Record, err error) {
	log.Error(context.Background(), starterTag, err,
		log.Int("id", rec.ID),
		log.String("destination", rec.Destination),
		log.Int("attempts", rec.Attempts+1),
		log.Msg("outbox: record dead after attempts"))
}
