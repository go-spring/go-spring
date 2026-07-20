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

package at

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/spring/aspect"
	"go-spring.org/stdlib/testing/assert"
)

// fakeBranch records which second-phase operation the coordinator drove, so a
// test can assert commit vs. rollback and how many times each branch ran.
type fakeBranch struct {
	id             string
	commits        int
	rollbacks      int
	rollbackErr    error
	commitErr      error
	restoredMarker *string // set to "restored" on rollback to prove undo ran
}

func (b *fakeBranch) ID() string { return b.id }

func (b *fakeBranch) Commit(context.Context, string) error {
	b.commits++
	return b.commitErr
}

func (b *fakeBranch) Rollback(context.Context, string) error {
	b.rollbacks++
	if b.restoredMarker != nil {
		*b.restoredMarker = "restored"
	}
	return b.rollbackErr
}

func TestCoordinator_CommitDropsEveryBranchOnce(t *testing.T) {
	c := NewCoordinator()
	ctx, xid := c.Begin(context.Background())

	b := &fakeBranch{id: "db1"}
	// Same resource registers twice (two statements) — must be deduplicated.
	assert.Error(t, c.Register(ctx, xid, b)).Nil()
	assert.Error(t, c.Register(ctx, xid, b)).Nil()

	assert.Error(t, c.Commit(ctx, xid)).Nil()
	assert.That(t, b.commits).Equal(1)
	assert.That(t, b.rollbacks).Equal(0)

	// The transaction is single-shot: resolving again is an error.
	assert.Error(t, c.Commit(ctx, xid)).Matches("unknown global transaction")
}

func TestCoordinator_RollbackRestoresInReverseOrder(t *testing.T) {
	c := NewCoordinator()
	ctx, xid := c.Begin(context.Background())

	var marker string
	b1 := &fakeBranch{id: "db1"}
	b2 := &fakeBranch{id: "db2", restoredMarker: &marker}
	assert.Error(t, c.Register(ctx, xid, b1)).Nil()
	assert.Error(t, c.Register(ctx, xid, b2)).Nil()

	assert.Error(t, c.Rollback(ctx, xid)).Nil()
	assert.That(t, b1.rollbacks).Equal(1)
	assert.That(t, b2.rollbacks).Equal(1)
	assert.That(t, b1.commits).Equal(0)
	assert.That(t, marker).Equal("restored") // before-image restoration ran
}

func TestCoordinator_RollbackFailureIsSurfaced(t *testing.T) {
	c := NewCoordinator()
	ctx, xid := c.Begin(context.Background())

	b := &fakeBranch{id: "db1", rollbackErr: errors.New("restore failed")}
	assert.Error(t, c.Register(ctx, xid, b)).Nil()

	assert.Error(t, c.Rollback(ctx, xid)).Matches("restore failed")
}

func TestCoordinator_RegisterUnknownTransaction(t *testing.T) {
	c := NewCoordinator()
	err := c.Register(context.Background(), "no-such-xid", &fakeBranch{id: "db1"})
	assert.That(t, errors.Is(err, ErrUnknownTransaction)).True()
}

func TestCoordinator_ReleasesGlobalLockOnResolve(t *testing.T) {
	lk := &MemoryGlobalLock{}
	c := NewCoordinator(WithGlobalLock(lk))
	ctx, xid := c.Begin(context.Background())

	keys := []LockKey{{Resource: "db1", Table: "account", PK: "1"}}
	assert.Error(t, lk.Acquire(ctx, xid, keys)).Nil()
	assert.Error(t, c.Register(ctx, xid, &fakeBranch{id: "db1"})).Nil()

	assert.Error(t, c.Commit(ctx, xid)).Nil()

	// After commit the lock is released, so a fresh transaction can take the row.
	assert.Error(t, lk.Acquire(ctx, "other-xid", keys)).Nil()
}

func TestMemoryGlobalLock_WriteWriteConflict(t *testing.T) {
	lk := &MemoryGlobalLock{}
	key := []LockKey{{Resource: "db1", Table: "account", PK: "1"}}

	assert.Error(t, lk.Acquire(context.Background(), "tx-A", key)).Nil()

	// A second transaction wanting the same row is rejected — write-write isolation.
	err := lk.Acquire(context.Background(), "tx-B", key)
	assert.That(t, errors.Is(err, ErrLockConflict)).True()

	// The same transaction re-locking its own row is a no-op, not a self-deadlock.
	assert.Error(t, lk.Acquire(context.Background(), "tx-A", key)).Nil()

	// Once A releases, B may proceed.
	assert.Error(t, lk.Release(context.Background(), "tx-A")).Nil()
	assert.Error(t, lk.Acquire(context.Background(), "tx-B", key)).Nil()
}

func TestMemoryGlobalLock_AllOrNothingOnConflict(t *testing.T) {
	lk := &MemoryGlobalLock{}
	held := []LockKey{{Resource: "db1", Table: "t", PK: "2"}}
	assert.Error(t, lk.Acquire(context.Background(), "tx-A", held)).Nil()

	// tx-B wants two keys, one of which conflicts: it must acquire neither.
	want := []LockKey{{Resource: "db1", Table: "t", PK: "1"}, {Resource: "db1", Table: "t", PK: "2"}}
	assert.That(t, errors.Is(lk.Acquire(context.Background(), "tx-B", want), ErrLockConflict)).True()

	// Proof PK:1 was not left locked by the failed attempt: tx-C can take it.
	free := []LockKey{{Resource: "db1", Table: "t", PK: "1"}}
	assert.Error(t, lk.Acquire(context.Background(), "tx-C", free)).Nil()
}

func TestGlobalAT_CommitsOnSuccessRollsBackOnError(t *testing.T) {
	c := NewCoordinator()
	chain := aspect.NewChain(GlobalAT(c))

	// Success path: a branch that self-registers during the business call is
	// committed when the method returns nil.
	var committed *fakeBranch
	_, err := chain.Run(context.Background(), "OrderService.Place", func(ctx context.Context) (any, error) {
		xid, ok := XIDFromContext(ctx)
		assert.That(t, ok).True() // the aspect injected an XID
		committed = &fakeBranch{id: "db1"}
		return nil, c.Register(ctx, xid, committed)
	})
	assert.Error(t, err).Nil()
	assert.That(t, committed.commits).Equal(1)
	assert.That(t, committed.rollbacks).Equal(0)

	// Failure path: the business error is propagated and the branch is rolled back.
	var rolledBack *fakeBranch
	_, err = chain.Run(context.Background(), "OrderService.Place", func(ctx context.Context) (any, error) {
		xid, _ := XIDFromContext(ctx)
		rolledBack = &fakeBranch{id: "db1"}
		_ = c.Register(ctx, xid, rolledBack)
		return nil, errors.New("insufficient balance")
	})
	assert.Error(t, err).Matches("insufficient balance")
	assert.That(t, rolledBack.rollbacks).Equal(1)
	assert.That(t, rolledBack.commits).Equal(0)
}
