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

// Package at defines a framework-agnostic, zero-dependency abstraction for the
// AT (Automatic Transaction) distributed-transaction pattern. It is the
// Go-idiomatic equivalent of Seata AT, reached without replicating Seata's
// TC/TM/RM roles.
//
// # Why a separate pattern from Saga and TCC
//
// [go-spring.org/spring/transaction] (Saga) and [go-spring.org/spring/transaction/tcc]
// (TCC) both require the developer to write the reverse operation by hand: a
// Saga step supplies a Compensate function, a TCC participant supplies Cancel.
// AT removes that burden. A branch's compensation is *derived automatically*
// from a before-image captured by intercepting the branch's DML: the framework
// records what each row looked like before the write, and on rollback restores
// exactly that. Business code writes only the forward SQL, exactly as if there
// were no distributed transaction at all — this is AT's defining property.
//
// AT also, unlike Saga, offers write-write isolation through a global row lock
// ([GlobalLock]): while a global transaction holds the lock on a row, another
// global transaction that touches the same row is rejected, so two in-flight
// transactions cannot both stage conflicting changes to one row.
//
// # The protocol
//
//   - A [Coordinator] begins a global transaction, assigning an id (the XID)
//     that rides the context ([WithXID]).
//   - As business code runs its forward DML, a resource-side interceptor (a GORM
//     plugin, in the bundled backend) captures a before/after [RecordImage] for
//     each statement, writes an [UndoLog] alongside the business data in the same
//     local transaction, acquires the [GlobalLock] for the affected rows, and
//     registers its [Branch] with the coordinator via [Coordinator.Register].
//   - On success the coordinator commits: every branch drops its undo logs and
//     the global lock is released — phase two is cheap because phase one already
//     committed the business data locally.
//   - On failure the coordinator rolls back: every branch restores its rows from
//     the recorded before-images (undoing what phase one committed) and the
//     global lock is released.
//
// # Layering
//
// The orchestration is backend-neutral and lives here with zero dependencies:
// the image capture, undo-log persistence and row restoration are inherently
// SQL/ORM specific, so they live behind the [Branch] seam implemented by a
// starter (see starter-transaction-at-gorm). The bundled [MemoryGlobalLock]
// keeps the framework standalone. Observability is contributed through the
// [Observer] seam so a starter can attach otel spans without pulling otel into
// stdlib.
package at

import (
	"context"

	"go-spring.org/spring/resilience"
)

// RetryPolicy governs how a branch's second-phase operation (commit or
// rollback) is retried on failure. It reuses [resilience.Policy] rather than
// inventing a second knob set, so the same declarative fields (MaxRetries,
// Timeout, ...) govern both outbound resilience and AT phase retries. The zero
// value means a single attempt with no retry.
type RetryPolicy = resilience.Policy

// SQLType is the kind of DML a branch captured, which determines how a row is
// restored on rollback: an inserted row is deleted, a deleted row is
// re-inserted, an updated row is set back to its before-image.
type SQLType int

const (
	// SQLInsert is a row insertion; its rollback deletes the inserted row.
	SQLInsert SQLType = iota
	// SQLUpdate is a row update; its rollback writes the before-image back.
	SQLUpdate
	// SQLDelete is a row deletion; its rollback re-inserts the deleted row.
	SQLDelete
)

// String renders the SQL type for logs and spans.
func (t SQLType) String() string {
	switch t {
	case SQLInsert:
		return "INSERT"
	case SQLUpdate:
		return "UPDATE"
	case SQLDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

// RowImage is a single row snapshot: its primary-key columns (used to locate the
// row on restore) plus every column value at the time of capture.
type RowImage struct {
	// PrimaryKeys identifies the row: column name to value. Restoration targets a
	// row by these, so they must be present for every image except an insert's
	// (empty) before-image.
	PrimaryKeys map[string]any

	// Values holds the row's column values at capture time. A before-image carries
	// the pre-write values (used to restore), an after-image the post-write values.
	Values map[string]any
}

// RecordImage is the set of rows one DML statement touched, in one table.
type RecordImage struct {
	// Table is the affected table name.
	Table string

	// Rows are the affected rows' images.
	Rows []RowImage
}

// UndoLog is the record a branch persists for one DML statement so the branch
// can undo it on rollback. It is written alongside the business data in the same
// local transaction, so business change and undo log commit atomically.
type UndoLog struct {
	// XID is the global transaction id this statement belongs to.
	XID string

	// BranchID identifies the resource (database) the statement ran against.
	BranchID string

	// Table is the affected table.
	Table string

	// SQLType is the kind of statement, which selects the restore strategy.
	SQLType SQLType

	// Before is the row image prior to the statement (empty for an insert).
	Before RecordImage

	// After is the row image after the statement (empty for a delete).
	After RecordImage
}

// LockKey identifies one row for the [GlobalLock]: the resource (database), its
// table and the row's primary key rendered as a string. Two writes conflict when
// they target the same LockKey.
type LockKey struct {
	Resource string
	Table    string
	PK       string
}

// String renders the key for logging and as a map key fallback.
func (k LockKey) String() string {
	return k.Resource + ":" + k.Table + ":" + k.PK
}

// Status is the outcome of a global transaction's second phase.
type Status int

const (
	// StatusBegun means the global transaction was started but not yet resolved.
	StatusBegun Status = iota
	// StatusCommitted means every branch committed (undo logs dropped).
	StatusCommitted
	// StatusRolledBack means every branch restored its before-images.
	StatusRolledBack
	// StatusCommitFailed means at least one branch failed to drop its undo logs.
	// The business data is already committed, so this is a cleanup failure, not a
	// consistency loss; the undo logs are left for retry / manual cleanup.
	StatusCommitFailed
	// StatusRollbackFailed means at least one branch failed to restore its rows.
	// The system may be left inconsistent and needs alerting / manual intervention.
	StatusRollbackFailed
)

// String renders the status for logs and spans.
func (s Status) String() string {
	switch s {
	case StatusBegun:
		return "Begun"
	case StatusCommitted:
		return "Committed"
	case StatusRolledBack:
		return "RolledBack"
	case StatusCommitFailed:
		return "CommitFailed"
	case StatusRollbackFailed:
		return "RollbackFailed"
	default:
		return "Unknown"
	}
}

// Phase distinguishes a branch's two second-phase operations, used by [Observer]
// to label what was running.
type Phase int

const (
	// PhaseCommit is the drop-undo-logs operation.
	PhaseCommit Phase = iota
	// PhaseRollback is the restore-from-before-image operation.
	PhaseRollback
)

// String renders the phase for logs and spans.
func (p Phase) String() string {
	switch p {
	case PhaseCommit:
		return "Commit"
	case PhaseRollback:
		return "Rollback"
	default:
		return "Unknown"
	}
}

// Branch is one resource (typically a database) participating in a global AT
// transaction. It is implemented by a backend starter that knows how to persist
// undo logs and restore rows for its concrete ORM/driver; the [Coordinator]
// drives it without any SQL knowledge of its own.
//
// Both second-phase operations must be idempotent: a crash or retry may replay
// them, and Rollback in particular may run against rows already restored.
type Branch interface {
	// ID identifies the resource. The coordinator deduplicates branches by it, so
	// a database that ran several statements in one global transaction is committed
	// or rolled back exactly once.
	ID() string

	// Commit finalizes this branch for xid by dropping the undo logs it recorded.
	// The business data was already committed locally in phase one, so this only
	// discards the now-unneeded undo information.
	Commit(ctx context.Context, xid string) error

	// Rollback undoes this branch for xid by restoring every row it changed from
	// the recorded before-image (delete inserts, re-insert deletes, revert
	// updates), then dropping the undo logs. It must be idempotent.
	Rollback(ctx context.Context, xid string) error
}

// GlobalLock provides AT's write-write isolation: a global transaction holds the
// lock on every row it writes for the duration of the transaction, so a second
// transaction that tries to write the same row is rejected with [ErrLockConflict]
// rather than silently staging a conflicting change. It is the in-process /
// distributed equivalent of Seata's global row lock.
//
// Implementations must be safe for concurrent use. The bundled [MemoryGlobalLock]
// is a zero-dependency in-process implementation; a distributed deployment
// supplies a shared (redis/db-backed) one.
type GlobalLock interface {
	// Acquire locks every key for xid. If any key is already held by a *different*
	// xid it returns [ErrLockConflict] and acquires none (all-or-nothing), so the
	// caller's write fails and its global transaction rolls back. Re-acquiring a
	// key already held by the same xid is a no-op, so a transaction that touches a
	// row twice does not deadlock against itself.
	Acquire(ctx context.Context, xid string, keys []LockKey) error

	// Release frees every key held by xid. It is idempotent.
	Release(ctx context.Context, xid string) error
}

// Coordinator orchestrates one global AT transaction. The bundled in-process
// implementation (see [NewCoordinator]) tracks the branches that register during
// phase one and, in phase two, commits or rolls back each exactly once and
// releases the global lock. Implementations must be safe for concurrent use.
type Coordinator interface {
	// Begin starts a global transaction, returning a context carrying the new XID
	// (see [WithXID]) and the XID itself. Resource interceptors read the XID from
	// the context to know they are inside a global transaction.
	Begin(ctx context.Context) (context.Context, string)

	// Register enrols a branch under xid. It is called by a resource interceptor
	// the first time that resource writes within the transaction; later writes from
	// the same resource are deduplicated by [Branch.ID]. Registering under an
	// unknown xid is an error.
	Register(ctx context.Context, xid string, b Branch) error

	// Commit resolves xid successfully: every registered branch drops its undo
	// logs and the global lock is released. A branch commit failure is collected
	// and reported (StatusCommitFailed) but does not stop the others.
	Commit(ctx context.Context, xid string) error

	// Rollback resolves xid by undoing it: every registered branch restores its
	// rows from the before-images and the global lock is released. A branch
	// rollback failure is collected and reported (StatusRollbackFailed) but does
	// not stop the others.
	Rollback(ctx context.Context, xid string) error
}

// Observer is the observability seam. The coordinator calls [Observer.Begin]
// around every branch's second-phase operation; the returned end function is
// invoked with the operation's error (nil on success). A starter implements this
// to open an otel span per branch phase without stdlib depending on otel. A nil
// Observer disables observation entirely.
type Observer interface {
	// Begin is called just before a branch phase runs. The returned context is used
	// for that phase (so a span can propagate through ctx); the returned end func is
	// called exactly once when the phase finishes.
	Begin(ctx context.Context, xid, branch string, phase Phase) (context.Context, func(err error))
}
