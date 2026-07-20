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
	"fmt"
	"reflect"
	"sort"
	"time"

	"go-spring.org/spring/transaction/at"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// suppressKey marks a context on which AT interception must not run — used by a
// branch's own second-phase writes (undo-log deletes, row restoration) so they
// are not themselves captured as new undo logs.
type suppressKey struct{}

// withSuppressed returns a context on which the AT callbacks are inert.
func withSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, suppressKey{}, true)
}

// atPlugin is the gorm.Plugin that turns a *gorm.DB into an AT branch resource.
// Installed with db.Use, it registers create/update/delete callbacks that, when
// the statement runs inside a global transaction ([at.WithXID] present on the
// context), capture an undo log, acquire the global row lock and register a
// [at.Branch] with the coordinator.
type atPlugin struct {
	resource string // branch id; distinguishes this database from others
	coord    at.Coordinator
	lock     at.GlobalLock
	db       *gorm.DB // base handle the branch uses for second-phase ops
}

// NewPlugin returns a gorm.Plugin that enables AT on the database it is installed
// on. resource is the branch id recorded in undo logs and lock keys (use a
// distinct value per database, e.g. "account-db"). coord and lock are the beans
// this starter contributes. Install it with db.Use(NewPlugin(...)) after calling
// [Migrate] on the same database.
//
//	if err := atgorm.Migrate(db); err != nil { ... }
//	if err := db.Use(atgorm.NewPlugin("account-db", coord, lock)); err != nil { ... }
func NewPlugin(resource string, coord at.Coordinator, lock at.GlobalLock) gorm.Plugin {
	return &atPlugin{resource: resource, coord: coord, lock: lock}
}

// Name implements gorm.Plugin.
func (p *atPlugin) Name() string { return "at:" + p.resource }

// Initialize implements gorm.Plugin, wiring the capture callbacks and pinning the
// base handle used for second-phase commit/rollback.
func (p *atPlugin) Initialize(db *gorm.DB) error {
	p.db = db
	cb := db.Callback()
	if err := cb.Update().Before("gorm:update").Register("at:before_update", p.beforeMutate(at.SQLUpdate)); err != nil {
		return err
	}
	if err := cb.Update().After("gorm:update").Register("at:after_update", p.afterMutate(at.SQLUpdate)); err != nil {
		return err
	}
	if err := cb.Delete().Before("gorm:delete").Register("at:before_delete", p.beforeMutate(at.SQLDelete)); err != nil {
		return err
	}
	if err := cb.Delete().After("gorm:delete").Register("at:after_delete", p.afterMutate(at.SQLDelete)); err != nil {
		return err
	}
	if err := cb.Create().After("gorm:create").Register("at:after_create", p.afterMutate(at.SQLInsert)); err != nil {
		return err
	}
	return nil
}

// active reports the global transaction id and whether this statement should be
// captured: it must be inside a global transaction, not suppressed, not the
// undo-log table itself, and have a resolvable schema.
func (p *atPlugin) active(db *gorm.DB) (string, bool) {
	ctx := db.Statement.Context
	if _, suppressed := ctx.Value(suppressKey{}).(bool); suppressed {
		return "", false
	}
	if tableName(db) == undoTableName {
		return "", false
	}
	xid, ok := at.XIDFromContext(ctx)
	if !ok {
		return "", false
	}
	return xid, true
}

// beforeMutate captures the before-image of an update/delete and acquires the
// global lock on the affected rows before the write runs. On a lock conflict it
// fails the statement, so the caller's transaction rolls back rather than staging
// a change that conflicts with an in-flight global transaction.
func (p *atPlugin) beforeMutate(sqlType at.SQLType) func(*gorm.DB) {
	return func(db *gorm.DB) {
		xid, ok := p.active(db)
		if !ok {
			return
		}
		table := tableName(db)
		where := whereExpr(db)
		before, err := p.selectImage(db, table, where)
		if err != nil {
			_ = db.AddError(err)
			return
		}
		if err := p.lock.Acquire(db.Statement.Context, xid, lockKeys(p.resource, before)); err != nil {
			_ = db.AddError(err)
			return
		}
		db.InstanceSet(beforeImageKeyString(sqlType), before)
	}
}

// afterMutate writes the undo log and registers the branch once the statement has
// run. For inserts the after-image (with generated keys) is captured here and the
// lock acquired now, since no before state existed to lock earlier.
func (p *atPlugin) afterMutate(sqlType at.SQLType) func(*gorm.DB) {
	return func(db *gorm.DB) {
		xid, ok := p.active(db)
		if !ok || db.Error != nil {
			return
		}
		table := tableName(db)

		var before, after at.RecordImage
		if v, ok := db.InstanceGet(beforeImageKeyString(sqlType)); ok {
			before, _ = v.(at.RecordImage)
		}
		switch sqlType {
		case at.SQLUpdate:
			// Re-read the changed rows so the after-image reflects the committed values.
			after, _ = p.selectImage(db, table, whereFromPKs(before))
		case at.SQLInsert:
			after = insertedImage(db, table)
			if err := p.lock.Acquire(db.Statement.Context, xid, lockKeys(p.resource, after)); err != nil {
				_ = db.AddError(err)
				return
			}
		}

		// A delete with no matched rows, or an update that changed nothing, has no
		// undo to record.
		if len(before.Rows) == 0 && len(after.Rows) == 0 {
			return
		}

		ctxJSON, err := encodeContext(before, after)
		if err != nil {
			_ = db.AddError(err)
			return
		}
		row := undoRow{
			XID:       xid,
			BranchID:  p.resource,
			Table:     table,
			SQLType:   int(sqlType),
			Context:   ctxJSON,
			CreatedAt: time.Now(),
		}
		// Write the undo log on the same connection/transaction as the business data
		// so the two commit atomically. NewDB gives a clean statement; the table-name
		// guard in active() stops this insert from recursing into capture. The context
		// is passed through the Session config, not by assigning sess.Statement.Context
		// — a Session{NewDB:true} initially shares db's Statement pointer, so mutating
		// it directly would leak state back onto the running statement.
		sess := db.Session(&gorm.Session{NewDB: true, Context: db.Statement.Context})
		if err := sess.Create(&row).Error; err != nil {
			_ = db.AddError(err)
			return
		}

		if err := p.coord.Register(db.Statement.Context, xid, &gormBranch{resource: p.resource, db: p.db}); err != nil {
			_ = db.AddError(err)
		}
	}
}

// selectImage reads the rows matched by where in table into a [at.RecordImage].
// It runs on the statement's own connection (a NewDB session off db), so within a
// business transaction it sees that transaction's state. The suppressed context
// is supplied through the Session config rather than by assigning
// sess.Statement.Context, because a Session{NewDB:true} initially shares db's
// Statement pointer and mutating it would leak the suppress flag onto the running
// statement (making its own after-callback skip capture).
func (p *atPlugin) selectImage(db *gorm.DB, table string, where clause.Expression) (at.RecordImage, error) {
	sess := db.Session(&gorm.Session{NewDB: true, Context: withSuppressed(db.Statement.Context)})
	q := sess.Table(table)
	if where != nil {
		q = q.Clauses(where)
	}
	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil {
		return at.RecordImage{}, err
	}
	return recordImage(table, primaryKeyNames(db), rows), nil
}

// tableName returns the statement's table, falling back to the schema table.
func tableName(db *gorm.DB) string {
	if db.Statement.Table != "" {
		return db.Statement.Table
	}
	if db.Statement.Schema != nil {
		return db.Statement.Schema.Table
	}
	return ""
}

// primaryKeyNames returns the DB column names of the statement's primary keys,
// defaulting to "id" when the schema does not declare one.
func primaryKeyNames(db *gorm.DB) []string {
	var pks []string
	if db.Statement.Schema != nil {
		for _, f := range db.Statement.Schema.PrimaryFields {
			pks = append(pks, f.DBName)
		}
	}
	if len(pks) == 0 {
		pks = []string{"id"}
	}
	return pks
}

// whereExpr extracts the WHERE clause the statement was built with, so the
// before-image SELECT matches exactly the rows the write will touch.
func whereExpr(db *gorm.DB) clause.Expression {
	if c, ok := db.Statement.Clauses["WHERE"]; ok {
		return c.Expression
	}
	return nil
}

// whereFromPKs builds a WHERE matching the rows in img by their primary keys, so
// the after-image re-read targets exactly the rows the before-image captured.
func whereFromPKs(img at.RecordImage) clause.Expression {
	var ors []clause.Expression
	for _, r := range img.Rows {
		var ands []clause.Expression
		for col, val := range r.PrimaryKeys {
			ands = append(ands, clause.Eq{Column: col, Value: val})
		}
		if len(ands) > 0 {
			ors = append(ors, clause.And(ands...))
		}
	}
	if len(ors) == 0 {
		return nil
	}
	return clause.Where{Exprs: []clause.Expression{clause.Or(ors...)}}
}

// recordImage assembles a RecordImage from queried rows, extracting each row's
// primary-key columns for later targeting.
func recordImage(table string, pks []string, rows []map[string]any) at.RecordImage {
	img := at.RecordImage{Table: table}
	for _, r := range rows {
		pk := make(map[string]any, len(pks))
		for _, name := range pks {
			pk[name] = r[name]
		}
		img.Rows = append(img.Rows, at.RowImage{PrimaryKeys: pk, Values: r})
	}
	return img
}

// insertedImage reads the primary keys gorm wrote back into the destination after
// a create, so an inserted row can be located (and deleted) on rollback.
func insertedImage(db *gorm.DB, table string) at.RecordImage {
	img := at.RecordImage{Table: table}
	if db.Statement.Schema == nil {
		return img
	}
	rv := db.Statement.ReflectValue
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if pk := pkFromValue(db, rv.Index(i)); pk != nil {
				img.Rows = append(img.Rows, at.RowImage{PrimaryKeys: pk})
			}
		}
	default:
		if pk := pkFromValue(db, rv); pk != nil {
			img.Rows = append(img.Rows, at.RowImage{PrimaryKeys: pk})
		}
	}
	return img
}

// pkFromValue reads the primary-key columns gorm populated into a created row's
// reflect value (autoincrement ids are written back after INSERT), returning a
// column-name to value map, or nil when no key could be read.
func pkFromValue(db *gorm.DB, rv reflect.Value) map[string]any {
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if !rv.IsValid() || db.Statement.Schema == nil {
		return nil
	}
	pk := make(map[string]any)
	for _, f := range db.Statement.Schema.PrimaryFields {
		if v, zero := f.ValueOf(db.Statement.Context, rv); !zero {
			pk[f.DBName] = v
		}
	}
	if len(pk) == 0 {
		return nil
	}
	return pk
}

// lockKeys renders one LockKey per row from its primary keys.
func lockKeys(resource string, img at.RecordImage) []at.LockKey {
	keys := make([]at.LockKey, 0, len(img.Rows))
	for _, r := range img.Rows {
		keys = append(keys, at.LockKey{Resource: resource, Table: img.Table, PK: pkString(r.PrimaryKeys)})
	}
	return keys
}

// pkString renders a primary-key map deterministically for use as a lock-key /
// map key, sorting columns so composite keys are stable across calls.
func pkString(pk map[string]any) string {
	cols := make([]string, 0, len(pk))
	for k := range pk {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	s := ""
	for _, c := range cols {
		s += fmt.Sprintf("%s=%v;", c, pk[c])
	}
	return s
}

// beforeImageKeyString derives a per-sql-type InstanceSet key so overlapping
// callbacks do not clobber each other's stashed image.
func beforeImageKeyString(t at.SQLType) string { return "at:before:" + t.String() }
