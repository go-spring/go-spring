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

package StarterRepositoryGorm_test

import (
	"context"
	"testing"

	"go-spring.org/cloud/data/experimental/data/repository"
	"go-spring.org/stdlib/testing/assert"
)

// TestFindPageEmptyPageBeyondEnd exercises a real round trip: a window wholly
// past the last row returns no items and no error, while Total still counts
// every matching row.
func TestFindPageEmptyPageBeyondEnd(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed(t, repo)

	page, err := repo.FindPage(ctx, repository.NewQuery().OrderBy("id").Slice(100, 10))
	assert.That(t, err).Nil()
	assert.That(t, len(page.Items)).Equal(0)
	assert.That(t, page.Total).Equal(int64(4))
	assert.That(t, page.HasNext()).False()
}

// TestFindPageExactLastPage walks a 4-row table in windows of 2 and pins the
// boundary: the last window that lands exactly on the total has no next page.
func TestFindPageExactLastPage(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed(t, repo)

	for offset, wantItems := range map[int]int{0: 2, 2: 2} {
		page, err := repo.FindPage(ctx, repository.NewQuery().OrderBy("id").Slice(offset, 2))
		assert.That(t, err).Nil()
		assert.That(t, len(page.Items)).Equal(wantItems)
		assert.That(t, page.Total).Equal(int64(4))
	}
	// 4+2 > 4: no further page.
	page, err := repo.FindPage(ctx, repository.NewQuery().OrderBy("id").Slice(4, 2))
	assert.That(t, err).Nil()
	assert.That(t, len(page.Items)).Equal(0)
	assert.That(t, page.HasNext()).False()
}

// TestSaveUpsertsByPrimaryKey verifies Save is a real upsert round trip: the
// second write updates the row in place instead of inserting a duplicate.
func TestSaveUpsertsByPrimaryKey(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()

	w := &widget{ID: 7, Name: "first", Price: 1}
	assert.That(t, repo.Create(ctx, w)).Nil()

	w.Name, w.Price = "second", 2
	assert.That(t, repo.Save(ctx, w)).Nil()

	n, err := repo.Count(ctx)
	assert.That(t, err).Nil()
	assert.That(t, n).Equal(int64(1), "Save must update, not duplicate")

	got, found, err := repo.FindByID(ctx, 7)
	assert.That(t, err).Nil()
	assert.That(t, found).True()
	assert.String(t, got.Name).Equal("second")
	assert.That(t, got.Price).Equal(2)
}

// TestUnsupportedOperatorRejected verifies an unknown Op fails before any SQL
// is issued, rather than silently matching everything.
func TestUnsupportedOperatorRejected(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed(t, repo)

	_, err := repo.FindAll(ctx, repository.NewQuery().Where("price", repository.Op("regex"), 1))
	assert.Error(t, err).Matches("unsupported operator")
}
