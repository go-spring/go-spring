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
	"encoding/json"
	"time"

	"go-spring.org/cloud/messaging"
	"gorm.io/gorm"
)

// Publish inserts one outbox record inside the business transaction tx, so the
// message commits atomically with the surrounding business writes — the core of
// the transactional outbox pattern:
//
//	err := db.Transaction(func(tx *gorm.DB) error {
//	    if err := tx.Create(&order).Error; err != nil { return err }
//	    return StarterOutboxGorm.Publish(tx, "orders", order.ID, payload, nil)
//	})
//
// tx must be the transactional session (or a db already inside a transaction);
// calling it on a non-transactional db makes the insert its own transaction,
// which breaks the atomicity the pattern exists to provide.
//
// The relay picks the record up after commit and delivers it at-least-once to
// destination via the configured driver.
func Publish(tx *gorm.DB, destination, key string, payload []byte, headers map[string]string) error {
	var hdr string
	if len(headers) > 0 {
		b, err := json.Marshal(headers)
		if err != nil {
			return err
		}
		hdr = string(b)
	}
	row := outboxRow{
		Destination: destination,
		Key:         key,
		Payload:     payload,
		Headers:     hdr,
		Status:      "pending",
		NextRetryAt: time.Now(),
		CreatedAt:   time.Now(),
	}
	return tx.Create(&row).Error
}

// PublishMessage is [Publish] taking a [messaging.Message] envelope; its Key,
// Payload and Headers carry over, its Timestamp does not (CreatedAt is the
// outbox record's own time).
func PublishMessage(tx *gorm.DB, destination string, msg *messaging.Message) error {
	return Publish(tx, destination, msg.Key, msg.Payload, msg.Headers)
}
