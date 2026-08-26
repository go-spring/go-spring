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

package repository_test

import (
	"context"
	"testing"

	"go-spring.org/cloud/data/experimental/data/repository"
	"go-spring.org/stdlib/testing/assert"
)

func TestQueryBuildersCompose(t *testing.T) {
	q := repository.NewQuery().
		Where("a", repository.Eq, 1).
		Where("b", repository.Like, "x%").
		OrderBy("c").
		OrderByDesc("d").
		Slice(10, 5)

	assert.That(t, len(q.Filters)).Equal(2)
	assert.That(t, q.Filters[0]).Equal(repository.Cond{Field: "a", Op: repository.Eq, Value: 1})
	assert.That(t, q.Filters[1]).Equal(repository.Cond{Field: "b", Op: repository.Like, Value: "x%"})
	assert.That(t, len(q.Sort)).Equal(2)
	assert.That(t, q.Sort[1]).Equal(repository.Order{Field: "d", Desc: true})
	assert.That(t, q.Page).Equal(repository.Pageable{Offset: 10, Limit: 5})
}

// TestFindPageEmptyWindowBeyondEnd verifies a window fully past the last row
// yields an empty (not erroneous) page whose Total still counts all matches.
func TestFindPageEmptyWindowBeyondEnd(t *testing.T) {
	b := &fakeBackend{findAll: nil, countBy: 7}
	repo := repository.New[user, int64](b)

	page, err := repo.FindPage(context.Background(), repository.NewQuery().Slice(100, 10))
	assert.That(t, err).Nil()
	assert.That(t, len(page.Items)).Equal(0)
	assert.That(t, page.Total).Equal(int64(7))
	assert.That(t, page.HasNext()).False()
}

// TestPageHasNextBoundaries pins the exact boundary semantics of HasNext: false
// on the last full page, on a page that ends exactly at the total, and when the
// window is unbounded.
func TestPageHasNextBoundaries(t *testing.T) {
	cases := []struct {
		offset, limit int
		total         int64
		want          bool
	}{
		{0, 2, 5, true},  // first of three pages
		{2, 2, 5, true},  // one row remains after this window
		{4, 2, 5, false}, // window ends exactly at the total
		{4, 1, 5, false}, // last row is the window's last item
		{6, 2, 5, false}, // window starts at the total
		{0, 5, 5, false}, // whole result in one page
		{0, 0, 5, false}, // unbounded
		{0, 2, 0, false}, // empty result
	}
	for _, c := range cases {
		p := repository.Page[user]{Total: c.total, Offset: c.offset, Limit: c.limit}
		assert.That(t, p.HasNext()).Equal(c.want)
	}
}

// TestZeroQueryMatchesEverything documents that the zero Query carries no
// filters, no sort and no window — the backend sees "everything".
func TestZeroQueryMatchesEverything(t *testing.T) {
	b := &fakeBackend{}
	repo := repository.New[user, int64](b)
	_, err := repo.FindPage(context.Background(), repository.Query{})
	assert.That(t, err).Nil()
	assert.That(t, len(b.lastQuery.Filters)).Equal(0)
	assert.That(t, len(b.lastQuery.Sort)).Equal(0)
	assert.That(t, b.lastQuery.Page).Equal(repository.Pageable{})
}
