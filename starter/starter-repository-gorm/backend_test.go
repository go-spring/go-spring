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
	"time"

	reposgorm "go-spring.org/starter-repository-gorm"
	"go-spring.org/spring/repository"
	"go-spring.org/stdlib/testing/assert"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type widget struct {
	ID        int64 `gorm:"primaryKey;column:id"`
	Name      string
	Price     int
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
}

func (widget) TableName() string { return "widgets" }

func (w *widget) SetCreatedAt(t time.Time) { w.CreatedAt = t }
func (w *widget) SetUpdatedAt(t time.Time) { w.UpdatedAt = t }
func (w *widget) SetCreatedBy(who string)  { w.CreatedBy = who }

func newRepo(t *testing.T, opts ...repository.Option) repository.Repository[widget, int64] {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.That(t, err).Nil()
	sqlDB, err := db.DB()
	assert.That(t, err).Nil()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	assert.That(t, db.AutoMigrate(&widget{})).Nil()
	return reposgorm.For[widget, int64](db, "widgets", opts...)
}

func seed(t *testing.T, repo repository.Repository[widget, int64]) {
	t.Helper()
	rows := []*widget{
		{ID: 1, Name: "alpha", Price: 10},
		{ID: 2, Name: "beta", Price: 20},
		{ID: 3, Name: "gamma", Price: 30},
		{ID: 4, Name: "alto", Price: 40},
	}
	for _, w := range rows {
		assert.That(t, repo.Create(context.Background(), w)).Nil()
	}
}

func TestCRUD(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed(t, repo)

	got, found, err := repo.FindByID(ctx, 3)
	assert.That(t, err).Nil()
	assert.That(t, found).True()
	assert.String(t, got.Name).Equal("gamma")

	_, found, err = repo.FindByID(ctx, 99)
	assert.That(t, err).Nil()
	assert.That(t, found).False()

	ok, err := repo.ExistsByID(ctx, 1)
	assert.That(t, err).Nil()
	assert.That(t, ok).True()
	ok, _ = repo.ExistsByID(ctx, 99)
	assert.That(t, ok).False()

	n, err := repo.Count(ctx)
	assert.That(t, err).Nil()
	assert.That(t, n).Equal(int64(4))

	assert.That(t, repo.Delete(ctx, 1)).Nil()
	n, _ = repo.Count(ctx)
	assert.That(t, n).Equal(int64(3))
}

func TestFindAllFiltersAndSort(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed(t, repo)

	// price >= 20, ordered by price descending -> alto(40), gamma(30), beta(20).
	got, err := repo.FindAll(ctx, repository.NewQuery().
		Where("price", repository.Ge, 20).
		OrderByDesc("price"))
	assert.That(t, err).Nil()
	assert.That(t, len(got)).Equal(3)
	assert.String(t, got[0].Name).Equal("alto")
	assert.String(t, got[2].Name).Equal("beta")

	// LIKE with a caller-supplied wildcard: names starting with "al".
	got, err = repo.FindAll(ctx, repository.NewQuery().
		Where("name", repository.Like, "al%").
		OrderBy("name"))
	assert.That(t, err).Nil()
	assert.That(t, len(got)).Equal(2)
	assert.String(t, got[0].Name).Equal("alpha")

	// IN binds a slice.
	got, err = repo.FindAll(ctx, repository.NewQuery().
		Where("id", repository.In, []int64{2, 4}).
		OrderBy("id"))
	assert.That(t, err).Nil()
	assert.That(t, len(got)).Equal(2)
	assert.String(t, got[1].Name).Equal("alto")
}

func TestScalarOperators(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed(t, repo)

	cases := []struct {
		op    repository.Op
		value int
		want  int
	}{
		{repository.Eq, 20, 1},
		{repository.Ne, 20, 3},
		{repository.Gt, 20, 2},
		{repository.Ge, 20, 3},
		{repository.Lt, 30, 2},
		{repository.Le, 30, 3},
	}
	for _, c := range cases {
		got, err := repo.FindAll(ctx, repository.NewQuery().Where("price", c.op, c.value))
		assert.That(t, err).Nil()
		assert.That(t, len(got)).Equal(c.want, string(c.op))
	}
}

func TestFindPageTotalIndependentOfWindow(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed(t, repo)

	// Filter matches 3 rows; window returns only 2, but Total must be 3.
	page, err := repo.FindPage(ctx, repository.NewQuery().
		Where("price", repository.Ge, 20).
		OrderBy("price").
		Slice(0, 2))
	assert.That(t, err).Nil()
	assert.That(t, len(page.Items)).Equal(2)
	assert.That(t, page.Total).Equal(int64(3))
	assert.That(t, page.HasNext()).True()
	assert.String(t, page.Items[0].Name).Equal("beta")

	// Second window: the remainder, no next page.
	page, err = repo.FindPage(ctx, repository.NewQuery().
		Where("price", repository.Ge, 20).
		OrderBy("price").
		Slice(2, 2))
	assert.That(t, err).Nil()
	assert.That(t, len(page.Items)).Equal(1)
	assert.That(t, page.Total).Equal(int64(3))
	assert.That(t, page.HasNext()).False()
}

func TestInvalidFieldRejected(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seed(t, repo)

	// An injection attempt in the field name is rejected before any SQL runs.
	_, err := repo.FindAll(ctx, repository.NewQuery().
		Where("price; DROP TABLE widgets", repository.Eq, 1))
	assert.Error(t, err).Matches("invalid filter field")

	_, err = repo.FindAll(ctx, repository.NewQuery().OrderBy("1=1; --"))
	assert.Error(t, err).Matches("invalid sort field")

	// The table is intact.
	n, _ := repo.Count(ctx)
	assert.That(t, n).Equal(int64(4))
}

func TestAuditPopulatedThroughGorm(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	repo := newRepo(t,
		repository.WithClock(func() time.Time { return now }),
		repository.WithPrincipal(func(context.Context) string { return "tester" }),
	)
	ctx := context.Background()

	w := &widget{ID: 1, Name: "x", Price: 5}
	assert.That(t, repo.Create(ctx, w)).Nil()

	reloaded, found, err := repo.FindByID(ctx, 1)
	assert.That(t, err).Nil()
	assert.That(t, found).True()
	assert.String(t, reloaded.CreatedBy).Equal("tester")
	assert.That(t, reloaded.CreatedAt.UTC()).Equal(now)
	assert.That(t, reloaded.UpdatedAt.UTC()).Equal(now)
}
