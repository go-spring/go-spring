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

// Package StarterTransactionSagaGorm contributes a durable, gorm-backed
// [transaction.Store] to the Saga distributed-transaction capability, so an
// application can recover sagas a crash left in flight. It is enabled by a blank
// import plus one property:
//
//	import _ "go-spring.org/starter-transaction-saga-gorm"
//	# spring.transaction.saga.store=gorm
//
// It autowires an existing *gorm.DB — provided by whichever gorm driver starter
// the application already imports (mysql, postgres, ...) — and AutoMigrates the
// saga_snapshots table at construction, failing fast if that is not possible.
//
// Because starter-transaction-saga registers its in-memory default Store with
// gs.OnMissingBean, contributing this Store makes that default step aside: the
// coordinator then writes its saga log here, and the startup recovery Runner
// reads in-flight sagas back from here — turning on crash recovery without any
// change to business code.
package StarterTransactionSagaGorm

import (
	"go-spring.org/spring/gs"
	"go-spring.org/spring/cloud/transaction"
	"gorm.io/gorm"
)

func init() {
	// Register the durable Store only when explicitly selected, exported under the
	// transaction.Store interface so it satisfies OnMissingBean in the saga
	// starter. The *gorm.DB (second arg) is autowired from the container.
	gs.Provide(newGormStore, gs.TagArg("${spring.transaction.saga.gorm}"), gs.TagArg("")).
		Condition(gs.OnProperty("spring.transaction.saga.store").HavingValue("gorm")).
		Export(gs.As[transaction.Store]())
}

// newGormStore builds the durable store from the bound config and an autowired
// *gorm.DB, creating the saga_snapshots table if absent (fail-fast on error).
func newGormStore(_ gormConfig, db *gorm.DB) (transaction.Store, error) {
	if err := db.AutoMigrate(&sagaSnapshot{}); err != nil {
		return nil, err
	}
	return &gormStore{db: db}, nil
}
