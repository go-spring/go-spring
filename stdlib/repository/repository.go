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

// Package repository defines a framework-agnostic, zero-dependency generic
// data-access abstraction, so a CRUD-and-paging concern can be declared once
// against a domain type and served by any store (SQL via gorm, Mongo, an
// in-memory map) without the business code knowing which.
//
// It is the Go-idiomatic equivalent of Spring Data's CrudRepository +
// PagingAndSortingRepository — a small, ready-made set of persistence
// operations parameterised over the entity type and its id type — reached with
// Go generics instead of proxy-generated method-name parsing. There is no query
// DSL and no derived-query magic; a caller expresses "find these" with a plain
// [Query] value ([Cond] filters + [Order] sort + [Pageable] window), the
// Go-idiomatic stand-in for a Spring Data Specification.
//
// The layering mirrors [go-spring.org/stdlib/cache] and
// [go-spring.org/stdlib/batch]:
//
//   - [Repository] is the interface business code depends on.
//   - [Backend] is the single seam a store implements: it translates a [Query]
//     into that store's own query. Because a backend needs a live client
//     (a *gorm.DB, a Mongo collection), the seam is a plain interface swap — a
//     bean-type choice — not a package-level driver registry.
//   - [New] wraps a [Backend] into a [Repository], adding the store-neutral
//     concerns that belong above any single backend: audit-field population
//     ([Auditable]) and composing a [Page] from a list plus a count.
//
// This package imports no driver. The gorm implementation of [Backend] (and the
// repository.For factory) lives in starter-repository-gorm; a second store
// (Mongo) only needs another [Backend], leaving this abstraction untouched.
package repository

import (
	"context"
	"time"
)

// Repository is the store-neutral data-access surface business code depends on,
// parameterised over the entity type T and its identifier type ID. It bundles
// CRUD (the CrudRepository half) with sorted, paged reads (the
// PagingAndSortingRepository half). Implementations are obtained from [New] over
// a [Backend]; they must be safe for concurrent use when their backend is.
type Repository[T any, ID comparable] interface {
	// Create persists a new entity. It fills the create-time audit fields when
	// the entity is [Auditable]. The entity is passed by pointer so a backend
	// can write back a generated key or the applied audit values.
	Create(ctx context.Context, entity *T) error

	// Save persists an entity that may or may not already exist (an upsert), and
	// refreshes the modified-time audit field when the entity is [Auditable].
	Save(ctx context.Context, entity *T) error

	// FindByID returns the entity with the given id. found is false (with a nil
	// error) when no such entity exists — a miss is not an error.
	FindByID(ctx context.Context, id ID) (entity T, found bool, err error)

	// ExistsByID reports whether an entity with the given id exists, without
	// materialising it.
	ExistsByID(ctx context.Context, id ID) (bool, error)

	// Delete removes the entity with the given id. Deleting an absent id is not
	// an error.
	Delete(ctx context.Context, id ID) error

	// Count returns the total number of entities.
	Count(ctx context.Context) (int64, error)

	// FindAll returns the entities matching the [Query] (filters, sort, window).
	// The zero Query returns every entity. It does not compute a total; use
	// FindPage when the caller needs one.
	FindAll(ctx context.Context, q Query) ([]T, error)

	// FindPage returns the windowed items plus the total count of rows the
	// query's filters match (ignoring the window), as a [Page]. It is the
	// paginator-friendly form of FindAll.
	FindPage(ctx context.Context, q Query) (Page[T], error)
}

// Backend is the single seam a store implements to be usable as a [Repository].
// It is deliberately the same shape as [Repository] minus the store-neutral
// concerns [New] layers on (auditing, page composition), so an implementation is
// a thin translation of a [Query] into the store's own dialect and nothing more.
//
// The seam is an interface, not a package-level driver registry: a backend is
// bound to a live client (a *gorm.DB, a Mongo collection), so selecting one is a
// bean-type swap — the same choice [go-spring.org/stdlib/batch].JobRepository
// makes.
type Backend[T any, ID comparable] interface {
	// Create inserts a new entity. Audit fields have already been applied by the
	// generic repository when this is called.
	Create(ctx context.Context, entity *T) error
	// Save upserts an entity. Audit fields have already been applied.
	Save(ctx context.Context, entity *T) error
	// FindByID returns the entity for id, or found=false on a miss.
	FindByID(ctx context.Context, id ID) (entity T, found bool, err error)
	// ExistsByID reports whether id exists.
	ExistsByID(ctx context.Context, id ID) (bool, error)
	// Delete removes id (absent id is not an error).
	Delete(ctx context.Context, id ID) error
	// Count returns the total number of entities.
	Count(ctx context.Context) (int64, error)
	// FindAll returns the entities matching q's filters/sort/window.
	FindAll(ctx context.Context, q Query) ([]T, error)
	// CountBy returns the number of entities matching q's filters, ignoring its
	// window and sort. It backs the Total of [Repository.FindPage].
	CountBy(ctx context.Context, q Query) (int64, error)
}

// Option configures a [Repository] built by [New].
type Option func(*options)

type options struct {
	principal PrincipalFunc
	clock     Clock
}

// WithPrincipal sets the [PrincipalFunc] used to fill the CreatedBy audit field.
// Without it, CreatedBy is left empty (timestamps are still populated).
func WithPrincipal(fn PrincipalFunc) Option {
	return func(o *options) { o.principal = fn }
}

// WithClock overrides the clock used for audit timestamps. It exists mainly for
// deterministic tests; production wiring leaves the default [time.Now].
func WithClock(clock Clock) Option {
	return func(o *options) { o.clock = clock }
}

// New wraps a [Backend] into a [Repository], layering the store-neutral concerns
// on top: it applies [Auditable] audit fields before each write (so timestamps
// and CreatedBy are correct regardless of backend) and composes [FindPage] from
// the backend's FindAll + CountBy. It panics if backend is nil, since a nil
// backend can never serve a request and failing at wiring time is safer than on
// the first call.
func New[T any, ID comparable](backend Backend[T, ID], opts ...Option) Repository[T, ID] {
	if backend == nil {
		panic("repository: New with nil backend")
	}
	o := options{clock: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	if o.clock == nil {
		o.clock = time.Now
	}
	return &repo[T, ID]{backend: backend, principal: o.principal, clock: o.clock}
}

// repo is the generic [Repository] returned by [New]. It owns only the
// store-neutral concerns; every store-specific operation delegates to backend.
type repo[T any, ID comparable] struct {
	backend   Backend[T, ID]
	principal PrincipalFunc
	clock     Clock
}

func (r *repo[T, ID]) Create(ctx context.Context, entity *T) error {
	applyCreateAudit(ctx, entity, r.clock(), r.principal)
	return r.backend.Create(ctx, entity)
}

func (r *repo[T, ID]) Save(ctx context.Context, entity *T) error {
	applyUpdateAudit(entity, r.clock())
	return r.backend.Save(ctx, entity)
}

func (r *repo[T, ID]) FindByID(ctx context.Context, id ID) (T, bool, error) {
	return r.backend.FindByID(ctx, id)
}

func (r *repo[T, ID]) ExistsByID(ctx context.Context, id ID) (bool, error) {
	return r.backend.ExistsByID(ctx, id)
}

func (r *repo[T, ID]) Delete(ctx context.Context, id ID) error {
	return r.backend.Delete(ctx, id)
}

func (r *repo[T, ID]) Count(ctx context.Context) (int64, error) {
	return r.backend.Count(ctx)
}

func (r *repo[T, ID]) FindAll(ctx context.Context, q Query) ([]T, error) {
	return r.backend.FindAll(ctx, q)
}

func (r *repo[T, ID]) FindPage(ctx context.Context, q Query) (Page[T], error) {
	items, err := r.backend.FindAll(ctx, q)
	if err != nil {
		return Page[T]{}, err
	}
	total, err := r.backend.CountBy(ctx, q)
	if err != nil {
		return Page[T]{}, err
	}
	return Page[T]{
		Items:  items,
		Total:  total,
		Offset: q.Page.Offset,
		Limit:  q.Page.Limit,
	}, nil
}
