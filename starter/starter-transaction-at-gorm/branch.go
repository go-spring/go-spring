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

package StarterTransactionATGorm

import (
	"context"

	"go-spring.org/spring/cloud/transaction/at"
	"gorm.io/gorm"
)

// gormBranch is one database's participation in a global AT transaction. It
// implements [at.Branch]: on commit it drops the undo logs it recorded, on
// rollback it restores every changed row from the recorded before-image and then
// drops the undo logs. It holds the base *gorm.DB (not the request transaction,
// which is gone by phase two), and runs its writes on a suppressed context so its
// own restoration is not re-captured as new undo logs.
type gormBranch struct {
	resource string
	db       *gorm.DB
}

var _ at.Branch = (*gormBranch)(nil)

// ID identifies the resource; the coordinator deduplicates branches by it.
func (b *gormBranch) ID() string { return b.resource }

// Commit drops the undo logs for xid. Phase one already committed the business
// data locally, so there is nothing else to do.
func (b *gormBranch) Commit(ctx context.Context, xid string) error {
	return b.deleteLogs(ctx, xid)
}

// Rollback restores every row this branch changed under xid from the recorded
// before-image and then drops the undo logs. Undo logs are applied in reverse
// insertion order (newest first) so later statements are undone before earlier
// ones, mirroring reverse-order compensation. It is idempotent: once the logs are
// deleted a repeat is a no-op.
func (b *gormBranch) Rollback(ctx context.Context, xid string) error {
	ctx = withSuppressed(ctx)
	var rows []undoRow
	if err := b.db.WithContext(ctx).
		Where("xid = ?", xid).
		Order("id DESC").
		Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		if err := b.restore(ctx, r); err != nil {
			return err
		}
	}
	return b.deleteLogs(ctx, xid)
}

// restore reverses one undo log: an inserted row is deleted, a deleted row is
// re-inserted from its before-image, an updated row is set back to its
// before-image values.
func (b *gormBranch) restore(ctx context.Context, r undoRow) error {
	c, err := decodeContext(r.Context)
	if err != nil {
		return err
	}
	db := b.db.WithContext(ctx)
	switch at.SQLType(r.SQLType) {
	case at.SQLInsert:
		for _, ri := range c.After.Rows {
			if len(ri.PrimaryKeys) == 0 {
				continue
			}
			if err := db.Table(r.Table).Where(ri.PrimaryKeys).Delete(&undoRowDummy{}).Error; err != nil {
				return err
			}
		}
	case at.SQLDelete:
		for _, ri := range c.Before.Rows {
			values := cloneMap(ri.Values)
			if err := db.Table(r.Table).Create(values).Error; err != nil {
				return err
			}
		}
	case at.SQLUpdate:
		for _, ri := range c.Before.Rows {
			if len(ri.PrimaryKeys) == 0 {
				continue
			}
			if err := db.Table(r.Table).Where(ri.PrimaryKeys).Updates(ri.Values).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// deleteLogs removes the undo logs for xid on this branch.
func (b *gormBranch) deleteLogs(ctx context.Context, xid string) error {
	return b.db.WithContext(withSuppressed(ctx)).
		Where("xid = ?", xid).
		Delete(&undoRow{}).Error
}

// undoRowDummy is a throwaway model that lets gorm run a Delete against an
// explicit Table() without a real model type.
type undoRowDummy struct{}

// cloneMap copies a values map so gorm's Create cannot mutate the caller's image.
func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
