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

package transactionobserve

import (
	"testing"

	"go-spring.org/cloud/experimental/transaction"
	"go-spring.org/stdlib/testing/assert"
)

func TestSagaSpanName(t *testing.T) {
	assert.That(t, sagaSpanName(transaction.PhaseAction, "DeductInventory")).Equal("saga.action DeductInventory")
	assert.That(t, sagaSpanName(transaction.PhaseCompensate, "DeductInventory")).Equal("saga.compensate DeductInventory")
}

func TestTccSpanName(t *testing.T) {
	assert.That(t, tccSpanName(transaction.PhaseTry, "Stock")).Equal("tcc.try Stock")
	assert.That(t, tccSpanName(transaction.PhaseConfirm, "Stock")).Equal("tcc.confirm Stock")
	assert.That(t, tccSpanName(transaction.PhaseCancel, "Stock")).Equal("tcc.cancel Stock")
}

func TestAtSpanName(t *testing.T) {
	assert.That(t, atSpanName(transaction.PhaseCommit, "branch-1")).Equal("at.commit branch-1")
	assert.That(t, atSpanName(transaction.PhaseRollback, "branch-1")).Equal("at.rollback branch-1")
}
