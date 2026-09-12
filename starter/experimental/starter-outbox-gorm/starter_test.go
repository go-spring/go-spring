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
	"context"
	"testing"

	"go-spring.org/cloud/messaging"
	"go-spring.org/spring/gs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// errDriver is a messaging.Driver whose every method fails. The relay only
// publishes when the outbox table has pending records, and this test leaves the
// table empty, so no driver method is ever invoked — the driver is present only
// to prove the relay autowires a messaging.Driver bean (not a registry lookup).
type errDriver struct{}

func (errDriver) NewPublisher(context.Context, string) (messaging.Publisher, error) {
	return nil, errNoop
}

func (errDriver) NewSubscriber(context.Context, string, string) (messaging.Subscriber, error) {
	return nil, errNoop
}

// errNoop marks an intentionally-never-called driver method.
var errNoop = &noopErr{}

type noopErr struct{}

func (*noopErr) Error() string { return "noop: not called" }

// TestRelay_AutowiresMessagingDriverBean wires the outbox starter with a
// messaging.Driver provided purely as a container bean (the path a broker
// starter's exported messaging.Driver bean rides). If the relay still resolved
// its delivery side from the messaging registry by name, no bean would satisfy
// it and container startup would fail; now it must start cleanly.
func TestRelay_AutowiresMessagingDriverBean(t *testing.T) {
	gs.Provide(newTestDBProvider)
	gs.Provide(func() messaging.Driver { return errDriver{} })

	gs.Configure(func(app gs.App) {
		// Keep the poller idle so the empty table never triggers a driver call.
		app.Property("spring.outbox.instances.main.poll-interval", "1h")
	}).RunTest(t, func(s *struct{}) {
		// startup success is the assertion: the relay root bean's Init ran with
		// the injected messaging.Driver bean.
	})
}

// newTestDBProvider opens an in-memory sqlite database with the outbox table
// created, as a bean for the outbox relay to autowire.
func newTestDBProvider() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := Migrate(db); err != nil {
		panic(err)
	}
	return db
}
