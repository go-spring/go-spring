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

package repository

import (
	"context"
	"time"
)

// Auditable is the optional interface an entity implements to receive automatic
// audit-field population, the Go-idiomatic equivalent of Spring Data's
// @CreatedDate / @LastModifiedDate / @CreatedBy. All three setters live on one
// interface; an entity that does not track a given field simply lets its setter
// discard the value.
//
// The generic [Repository] calls these setters before a write, so audit fields
// stay backend-neutral: the same entity carries correct timestamps whether it
// is persisted to SQL, Mongo, or an in-memory store. A create fills all three;
// an update refreshes only UpdatedAt (created-by/at are immutable once set).
type Auditable interface {
	// SetCreatedAt records when the entity was first persisted.
	SetCreatedAt(t time.Time)
	// SetUpdatedAt records when the entity was last modified.
	SetUpdatedAt(t time.Time)
	// SetCreatedBy records the principal that first persisted the entity.
	SetCreatedBy(who string)
}

// PrincipalFunc resolves the current principal (the "who" for CreatedBy) from
// the request context. It is the seam that aligns auditing with authentication:
// wire it to read the same subject the security layer put on the context. When
// no PrincipalFunc is configured, CreatedBy is left empty.
type PrincipalFunc func(ctx context.Context) string

// Clock returns the current time. It is injectable so tests can pin audit
// timestamps deterministically; production wiring uses [time.Now].
type Clock func() time.Time

// applyCreateAudit fills the create-time audit fields on entity when it is
// [Auditable]. It is called by the generic repository before a create.
func applyCreateAudit(ctx context.Context, entity any, now time.Time, principal PrincipalFunc) {
	a, ok := entity.(Auditable)
	if !ok {
		return
	}
	a.SetCreatedAt(now)
	a.SetUpdatedAt(now)
	if principal != nil {
		a.SetCreatedBy(principal(ctx))
	}
}

// applyUpdateAudit refreshes only the modified-time audit field on entity when
// it is [Auditable]. Created-by/at are immutable after creation, so an update
// leaves them untouched.
func applyUpdateAudit(entity any, now time.Time) {
	if a, ok := entity.(Auditable); ok {
		a.SetUpdatedAt(now)
	}
}
