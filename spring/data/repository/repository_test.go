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
	"errors"
	"testing"
	"time"

	"go-spring.org/spring/data/repository"
	"go-spring.org/stdlib/testing/assert"
)

// user is the entity under test; it implements [repository.Auditable] so the
// generic repository fills its audit fields on write.
type user struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
}

func (u *user) SetCreatedAt(t time.Time) { u.CreatedAt = t }
func (u *user) SetUpdatedAt(t time.Time) { u.UpdatedAt = t }
func (u *user) SetCreatedBy(who string)  { u.CreatedBy = who }

// fakeBackend records what the generic repository handed it, so tests can assert
// on audit population and page composition without a real store. FindAll returns
// a fixed window; CountBy returns a fixed, independent total to prove FindPage
// keeps them separate.
type fakeBackend struct {
	created   *user
	saved     *user
	findAll   []user
	countBy   int64
	lastQuery repository.Query
	err       error
}

func (b *fakeBackend) Create(_ context.Context, e *user) error { b.created = e; return b.err }
func (b *fakeBackend) Save(_ context.Context, e *user) error   { b.saved = e; return b.err }

func (b *fakeBackend) FindByID(_ context.Context, id int64) (user, bool, error) {
	return user{ID: id, Name: "x"}, true, b.err
}

func (b *fakeBackend) ExistsByID(_ context.Context, _ int64) (bool, error) { return true, b.err }
func (b *fakeBackend) Delete(_ context.Context, _ int64) error             { return b.err }
func (b *fakeBackend) Count(_ context.Context) (int64, error)              { return b.countBy, b.err }

func (b *fakeBackend) FindAll(_ context.Context, q repository.Query) ([]user, error) {
	b.lastQuery = q
	return b.findAll, b.err
}

func (b *fakeBackend) CountBy(_ context.Context, _ repository.Query) (int64, error) {
	return b.countBy, b.err
}

func TestNewNilBackendPanics(t *testing.T) {
	assert.Panic(t, func() {
		repository.New[user, int64](nil)
	}, "nil backend")
}

func TestCreateAppliesAuditFields(t *testing.T) {
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	b := &fakeBackend{}
	repo := repository.New[user, int64](b,
		repository.WithClock(func() time.Time { return now }),
		repository.WithPrincipal(func(context.Context) string { return "alice" }),
	)

	u := &user{ID: 1, Name: "a"}
	assert.That(t, repo.Create(context.Background(), u)).Nil()

	assert.That(t, b.created).Same(u)
	assert.That(t, u.CreatedAt).Equal(now)
	assert.That(t, u.UpdatedAt).Equal(now)
	assert.String(t, u.CreatedBy).Equal("alice")
}

func TestSaveRefreshesOnlyUpdatedAt(t *testing.T) {
	created := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	b := &fakeBackend{}
	repo := repository.New[user, int64](b, repository.WithClock(func() time.Time { return updated }))

	// An already-created entity: CreatedAt/By must survive a Save untouched.
	u := &user{ID: 1, Name: "a", CreatedAt: created, CreatedBy: "bob"}
	assert.That(t, repo.Save(context.Background(), u)).Nil()

	assert.That(t, u.CreatedAt).Equal(created)
	assert.String(t, u.CreatedBy).Equal("bob")
	assert.That(t, u.UpdatedAt).Equal(updated)
}

func TestCreatedByEmptyWithoutPrincipal(t *testing.T) {
	b := &fakeBackend{}
	repo := repository.New[user, int64](b)
	u := &user{ID: 1}
	assert.That(t, repo.Create(context.Background(), u)).Nil()
	assert.String(t, u.CreatedBy).Equal("")
	assert.That(t, u.CreatedAt.IsZero()).False()
}

func TestFindPageComposesItemsAndTotal(t *testing.T) {
	b := &fakeBackend{
		findAll: []user{{ID: 1}, {ID: 2}},
		countBy: 57, // total is independent of the returned window
	}
	repo := repository.New[user, int64](b)

	q := repository.NewQuery().Where("name", repository.Like, "a%").OrderByDesc("id").Slice(20, 2)
	page, err := repo.FindPage(context.Background(), q)
	assert.That(t, err).Nil()

	assert.That(t, len(page.Items)).Equal(2)
	assert.That(t, page.Total).Equal(int64(57))
	assert.That(t, page.Offset).Equal(20)
	assert.That(t, page.Limit).Equal(2)
	assert.That(t, page.HasNext()).True() // 20+2 < 57

	// The query reached the backend verbatim.
	assert.That(t, len(b.lastQuery.Filters)).Equal(1)
	assert.That(t, b.lastQuery.Sort[0]).Equal(repository.Order{Field: "id", Desc: true})
}

func TestFindPagePropagatesError(t *testing.T) {
	b := &fakeBackend{err: errors.New("boom")}
	repo := repository.New[user, int64](b)
	_, err := repo.FindPage(context.Background(), repository.NewQuery())
	assert.Error(t, err).Matches("boom")
}

func TestPageHasNextUnboundedIsFalse(t *testing.T) {
	p := repository.Page[user]{Total: 100, Offset: 0, Limit: 0}
	assert.That(t, p.HasNext()).False()
}
