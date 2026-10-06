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

// Package StarterTransactionTccGorm contributes a durable, gorm-backed
// [tcc.Store] to the TCC distributed-transaction capability, so an application
// can recover transactions a crash left in flight. It is enabled by a blank
// import plus one property:
//
//	import _ "go-spring.org/starter-transaction-tcc-gorm"
//	# spring.transaction.tcc.store=gorm
//
// It autowires an existing *gorm.DB — provided by whichever gorm driver starter
// the application already imports (mysql, postgres, ...) — and AutoMigrates the
// tcc_snapshots table at construction, failing fast if that is not possible.
//
// Because starter-transaction-tcc registers its in-memory default Store with
// gs.OnMissingBean, contributing this Store makes that default step aside: the
// coordinator then writes its TCC log here, and the startup recovery Runner
// reads in-flight transactions back from here — turning on crash recovery
// without any change to business code.
package StarterTransactionTccGorm

import (
	"context"

	"go-spring.org/cloud/experimental/transaction/tcc"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"gorm.io/gorm"
)

func init() {
	// Register the durable Store only when explicitly selected, exported under the
	// tcc.Store interface so it satisfies OnMissingBean in the tcc starter. The
	// *gorm.DB (second arg) is autowired from the container.
	gs.Provide(newGormStore, gs.TagArg("${spring.transaction.tcc.gorm}"), gs.TagArg("")).
		Condition(gs.OnProperty("spring.transaction.tcc.store").HavingValue("gorm")).
		Export(gs.As[tcc.Store]())
}

// newGormStore builds the durable store from the bound config and an autowired
// *gorm.DB, creating the tcc_snapshots table if absent (fail-fast on error).
func newGormStore(_ gormConfig, db *gorm.DB) (tcc.Store, error) {
	if err := db.AutoMigrate(&tccSnapshot{}); err != nil {
		log.Errorf(context.Background(), log.TagAppDef, err, "auto-migrate tcc_snapshots failed")
		return nil, err
	}
	log.Infof(context.Background(), log.TagAppDef, "create gorm tcc store success")
	return &gormStore{db: db}, nil
}
