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

package StarterRepositoryGorm

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"go-spring.org/spring/repository"
	"gorm.io/gorm"
)

// identPattern guards every field name a [repository.Query] interpolates into a
// SQL clause. Field names come from developer code (a Cond.Field, an Order.Field),
// not end-user input, but validating them as plain identifiers is cheap insurance
// against a value ever reaching the WHERE/ORDER BY string uninspected — the values
// themselves always go through gorm's parameter binding.
var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// gormBackend is the gorm implementation of [repository.Backend]. It translates
// a [repository.Query] into gorm's chained builder (Where/Order/Offset/Limit)
// and scans results back into the entity type. It holds only the *gorm.DB, the
// table name, and the resolved primary-key column, so it is safe for concurrent
// use exactly as the underlying *gorm.DB is.
type gormBackend[T any, ID comparable] struct {
	db       *gorm.DB
	table    string
	pkColumn string
}

// resolvePrimaryKey parses the entity schema to find its primary-key column, so
// FindByID/ExistsByID/Delete can build an explicit WHERE <pk> = ? instead of
// relying on gorm's inline-id inference under an overridden table name. It falls
// back to "id" (the Spring Data convention) when the schema has no declared key.
func resolvePrimaryKey[T any](db *gorm.DB) string {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(new(T)); err == nil && stmt.Schema != nil {
		if f := stmt.Schema.PrioritizedPrimaryField; f != nil {
			return f.DBName
		}
	}
	return "id"
}

func (b *gormBackend[T, ID]) Create(ctx context.Context, entity *T) error {
	return b.db.WithContext(ctx).Table(b.table).Create(entity).Error
}

func (b *gormBackend[T, ID]) Save(ctx context.Context, entity *T) error {
	return b.db.WithContext(ctx).Table(b.table).Save(entity).Error
}

func (b *gormBackend[T, ID]) FindByID(ctx context.Context, id ID) (T, bool, error) {
	var out T
	err := b.db.WithContext(ctx).Table(b.table).
		Where(b.pkColumn+" = ?", id).Take(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var zero T
		return zero, false, nil
	}
	if err != nil {
		var zero T
		return zero, false, err
	}
	return out, true, nil
}

func (b *gormBackend[T, ID]) ExistsByID(ctx context.Context, id ID) (bool, error) {
	var n int64
	err := b.db.WithContext(ctx).Table(b.table).
		Where(b.pkColumn+" = ?", id).Limit(1).Count(&n).Error
	return n > 0, err
}

func (b *gormBackend[T, ID]) Delete(ctx context.Context, id ID) error {
	return b.db.WithContext(ctx).Table(b.table).
		Where(b.pkColumn+" = ?", id).Delete(new(T)).Error
}

func (b *gormBackend[T, ID]) Count(ctx context.Context) (int64, error) {
	var n int64
	err := b.db.WithContext(ctx).Table(b.table).Count(&n).Error
	return n, err
}

func (b *gormBackend[T, ID]) FindAll(ctx context.Context, q repository.Query) ([]T, error) {
	tx, err := applyFilters(b.db.WithContext(ctx).Table(b.table), q.Filters)
	if err != nil {
		return nil, err
	}
	if tx, err = applySort(tx, q.Sort); err != nil {
		return nil, err
	}
	tx = applyPage(tx, q.Page)
	var out []T
	if err := tx.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (b *gormBackend[T, ID]) CountBy(ctx context.Context, q repository.Query) (int64, error) {
	// Total ignores sort and window by design: it is the count of all rows the
	// filters match, which is what a paginator needs.
	tx, err := applyFilters(b.db.WithContext(ctx).Table(b.table), q.Filters)
	if err != nil {
		return 0, err
	}
	var n int64
	if err := tx.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// applyFilters ANDs each [repository.Cond] onto the builder. The value always
// rides through gorm's "?" parameter binding; only the field name and operator
// shape the SQL text, and the field name is validated first.
func applyFilters(tx *gorm.DB, filters []repository.Cond) (*gorm.DB, error) {
	for _, c := range filters {
		if !identPattern.MatchString(c.Field) {
			return nil, fmt.Errorf("repository-gorm: invalid filter field %q", c.Field)
		}
		clause, err := clauseFor(c.Field, c.Op)
		if err != nil {
			return nil, err
		}
		tx = tx.Where(clause, c.Value)
	}
	return tx, nil
}

// clauseFor maps an [repository.Op] to its SQL fragment for field. IN binds a
// slice value ("IN ?"), the others bind a scalar; LIKE expects the caller's
// value to already carry any wildcards.
func clauseFor(field string, op repository.Op) (string, error) {
	switch op {
	case repository.Eq:
		return field + " = ?", nil
	case repository.Ne:
		return field + " <> ?", nil
	case repository.Gt:
		return field + " > ?", nil
	case repository.Ge:
		return field + " >= ?", nil
	case repository.Lt:
		return field + " < ?", nil
	case repository.Le:
		return field + " <= ?", nil
	case repository.In:
		return field + " IN ?", nil
	case repository.Like:
		return field + " LIKE ?", nil
	default:
		return "", fmt.Errorf("repository-gorm: unsupported operator %q", op)
	}
}

// applySort appends each sort key as a validated "<field> ASC|DESC" ORDER BY term.
func applySort(tx *gorm.DB, sort []repository.Order) (*gorm.DB, error) {
	for _, o := range sort {
		if !identPattern.MatchString(o.Field) {
			return nil, fmt.Errorf("repository-gorm: invalid sort field %q", o.Field)
		}
		dir := " ASC"
		if o.Desc {
			dir = " DESC"
		}
		tx = tx.Order(o.Field + dir)
	}
	return tx, nil
}

// applyPage applies the offset/limit window. A non-positive Limit leaves the
// result unbounded; a non-positive Offset starts at the first row.
func applyPage(tx *gorm.DB, p repository.Pageable) *gorm.DB {
	if p.Offset > 0 {
		tx = tx.Offset(p.Offset)
	}
	if p.Limit > 0 {
		tx = tx.Limit(p.Limit)
	}
	return tx
}
