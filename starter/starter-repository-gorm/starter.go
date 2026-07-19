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

// Package StarterRepositoryGorm is the gorm-backed implementation of the
// framework-neutral [go-spring.org/stdlib/repository] abstraction: it translates
// a [repository.Query] into gorm's chained builder and returns a ready-to-use
// generic [repository.Repository] over any *gorm.DB.
//
// It is a library-first integration module, not a blank-import starter. There
// is nothing to auto-register, because a repository is parameterised over a
// domain type the application owns; the application decides which entities get
// one. The single entry point is [For]:
//
//	type UserService struct{ repo repository.Repository[User, int64] }
//
//	func newUserService(db *gorm.DB) *UserService {
//	    return &UserService{repo: reposgorm.For[User, int64](db, "users")}
//	}
//
// To live in the IoC container as a named bean other beans autowire by
// interface, register [For] through a plain gs.Provide constructor — the same
// chainable call every starter uses (no wrapper needed, so Condition/profiles
// stay available):
//
//	gs.Provide(func(db *gorm.DB) repository.Repository[User, int64] {
//	    return reposgorm.For[User, int64](db, "users",
//	        repository.WithPrincipal(currentUser))
//	}).Name("userRepo")
//
// The gorm driver (MySQL, Postgres, sqlite, ...) is chosen by whichever
// starter-gorm-* published the *gorm.DB bean; this module is driver-agnostic.
// A second store (Mongo) is a separate [repository.Backend] implementation and
// does not touch this package or the abstraction.
package StarterRepositoryGorm

import (
	"go-spring.org/stdlib/repository"
	"gorm.io/gorm"
)

// For builds a [repository.Repository] for entity type T (identified by ID)
// backed by the given *gorm.DB and table. The primary-key column is resolved
// from T's gorm schema (falling back to "id"); pass [repository.WithPrincipal]
// / [repository.WithClock] through opts to enable audit-field population.
//
// Call it wherever a *gorm.DB is in scope — inline in a service constructor, or
// inside a gs.Provide constructor to publish it as an IoC bean.
func For[T any, ID comparable](db *gorm.DB, table string, opts ...repository.Option) repository.Repository[T, ID] {
	backend := &gormBackend[T, ID]{
		db:       db,
		table:    table,
		pkColumn: resolvePrimaryKey[T](db),
	}
	return repository.New(backend, opts...)
}
