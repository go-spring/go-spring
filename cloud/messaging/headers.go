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

package messaging

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Reserved header keys every driver in this family honours. They are ordinary
// headers — producers and drivers read and write them with [Message.SetHeader]
// — but their meaning is fixed here so idempotency and delivery visibility
// survive a broker switch.
const (
	// HeaderMessageID is the message's unique id, set by the producer (see
	// [NewMessageID]). At-least-once delivery makes consumer-side
	// de-duplication a production must; a stable id is what it keys on.
	// Drivers map it to the broker's own message id where one exists (the
	// broker's value wins on consume) and always preserve it through
	// round-trips.
	HeaderMessageID = "message-id"

	// HeaderDeliveryAttempt is how many times the broker has delivered this
	// message, 1-based, filled by the driver on consume from the broker's own
	// redelivery count where one exists (RocketMQ reconsumeTimes, RabbitMQ
	// x-death, ...). Absent or "1" means first delivery. A consumer can read
	// it to decide "retry" vs "straight to the dead letter" without counting
	// in-process state that a restart would lose.
	HeaderDeliveryAttempt = "delivery-attempt"
)

// NewMessageID returns a fresh unique message id for [HeaderMessageID]:
// 32 hex characters from 16 random bytes. It is not a UUID — it only has to
// be unique enough to key de-duplication.
func NewMessageID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand never fails on the supported platforms; if it somehow
		// does, fall back to a time-based id rather than panic mid-publish.
		now := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(now >> (uint(i%8) * 8))
			now = now*6364136223846793005 + 1442695040888963407
		}
	}
	return hex.EncodeToString(b[:])
}

// EnsureMessageID stamps a fresh [NewMessageID] onto msg when its
// [HeaderMessageID] is empty, leaving an existing id untouched. Publishers call
// it before sending so every envelope carries a dedup key by default; a caller
// that owns the id (or deduplicates on a business key instead) just sets the
// header first and this is a no-op.
func EnsureMessageID(msg *Message) {
	if msg.Header(HeaderMessageID) == "" {
		msg.SetHeader(HeaderMessageID, NewMessageID())
	}
}
