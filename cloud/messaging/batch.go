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

import "context"

// BatchPublisher is the optional batch-send capability a Publisher may also
// implement. It is deliberately not part of [Publisher]: not every broker
// SDK exposes a native batch API, so the interface lives one assertion away
// and [BatchPublish] downgrades gracefully to one-by-one sends.
type BatchPublisher interface {
	// PublishBatch sends msgs in one broker-side batch when the broker
	// supports it. It fails atomically where the broker does and reports
	// partial results where the broker cannot.
	PublishBatch(ctx context.Context, msgs []*Message) error
}

// BatchPublish sends msgs through p in one call. When p also implements
// [BatchPublisher] the batch path is used; otherwise each message is
// published in order, stopping at the first error (earlier messages are
// already sent — BatchPublish is not transactional).
func BatchPublish(ctx context.Context, p Publisher, msgs ...*Message) error {
	if bp, ok := p.(BatchPublisher); ok {
		return bp.PublishBatch(ctx, msgs)
	}
	for _, m := range msgs {
		if err := p.Publish(ctx, m); err != nil {
			return err
		}
	}
	return nil
}
