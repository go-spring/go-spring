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

// Command example is the acceptance smoke test for starter-repository-gorm. It
// wires a repository.Repository[Person, int64] as an IoC bean over an in-memory
// sqlite *gorm.DB and drives the full surface the abstraction promises:
//
//   - CRUD: Create/FindByID/ExistsByID/Save/Delete/Count;
//   - paging + composite conditions: FindAll and FindPage with a Query that
//     ANDs filters, sorts, and windows the result;
//   - audit: the entity implements repository.Auditable, so CreatedAt/UpdatedAt
//     fill automatically and CreatedBy comes from the request context via the
//     PrincipalFunc seam.
//
// The service bean autowires the Repository by interface — it never learns the
// backend is gorm, which is the point of the abstraction. check.sh runs this and
// fails on any deviation (the program exits non-zero).
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/data/repository"
	"go-spring.org/spring/gs"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	reposgorm "go-spring.org/starter-repository-gorm"
)

// principalKey carries the current user on the context; the repository's
// PrincipalFunc reads it to fill the CreatedBy audit field, mirroring how the
// security layer would expose the authenticated subject.
type principalKey struct{}

func withUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, principalKey{}, user)
}

func currentUser(ctx context.Context) string {
	s, _ := ctx.Value(principalKey{}).(string)
	return s
}

// Person is the demo entity. It implements repository.Auditable so the generic
// repository populates the audit columns on write, with no per-field code.
type Person struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"column:name"`
	Age       int       `gorm:"column:age"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	CreatedBy string    `gorm:"column:created_by"`
}

func (Person) TableName() string { return "people" }

func (p *Person) SetCreatedAt(t time.Time) { p.CreatedAt = t }
func (p *Person) SetUpdatedAt(t time.Time) { p.UpdatedAt = t }
func (p *Person) SetCreatedBy(who string)  { p.CreatedBy = who }

// openDB opens an in-memory sqlite database and migrates the Person table. A
// shared-cache handle pinned to one connection keeps the in-memory data alive
// for the process.
func openDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file:repo?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&Person{}); err != nil {
		return nil, err
	}
	return db, nil
}

// PersonService depends only on the framework-neutral repository interface — it
// has no idea the store is gorm/sqlite. The bean is provided below through a
// plain gs.Provide constructor over reposgorm.For, the documented "register as
// a named bean" pattern.
type PersonService struct {
	Repo repository.Repository[Person, int64] `autowire:""`
}

func main() {
	// Publish the *gorm.DB bean.
	gs.Provide(openDB)

	// Register the repository as a named IoC bean, exported under the neutral
	// interface. Audit fields are enabled by wiring the PrincipalFunc seam to the
	// request context.
	gs.Provide(func(db *gorm.DB) repository.Repository[Person, int64] {
		return reposgorm.For[Person, int64](db, "people",
			repository.WithPrincipal(currentUser))
	}).Name("personRepo")

	svcBean := gs.Provide(&PersonService{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(svcBean.Interface().(*PersonService))
	}()

	gs.Run()
}

// runTest exercises CRUD, paging, composite conditions and audit population,
// asserting each expectation and exiting non-zero on the first deviation so
// check.sh fails loudly.
func runTest(s *PersonService) {
	repo := s.Repo
	ctx := withUser(context.Background(), "alice")

	// --- Create + audit -----------------------------------------------------
	people := []*Person{
		{ID: 1, Name: "Ann", Age: 20},
		{ID: 2, Name: "Bob", Age: 35},
		{ID: 3, Name: "Cate", Age: 42},
		{ID: 4, Name: "Dan", Age: 30},
	}
	for _, p := range people {
		if err := repo.Create(ctx, p); err != nil {
			fail("create %s: %v", p.Name, err)
		}
	}
	if people[0].CreatedBy != "alice" || people[0].CreatedAt.IsZero() {
		fail("audit not applied on create: createdBy=%q createdAt=%v",
			people[0].CreatedBy, people[0].CreatedAt)
	}
	fmt.Println("create + audit OK: 4 rows, createdBy=alice, timestamps set")

	// --- FindByID / ExistsByID ---------------------------------------------
	got, found, err := repo.FindByID(ctx, 2)
	if err != nil || !found || got.Name != "Bob" {
		fail("findByID(2): got=%+v found=%v err=%v", got, found, err)
	}
	if _, found, _ := repo.FindByID(ctx, 999); found {
		fail("findByID(999): expected miss")
	}
	if ok, _ := repo.ExistsByID(ctx, 3); !ok {
		fail("existsByID(3): expected true")
	}
	if ok, _ := repo.ExistsByID(ctx, 999); ok {
		fail("existsByID(999): expected false")
	}
	fmt.Println("findByID / existsByID OK")

	// --- Count --------------------------------------------------------------
	if n, _ := repo.Count(ctx); n != 4 {
		fail("count: got %d, want 4", n)
	}

	// --- Composite conditions + sort ---------------------------------------
	// age >= 30, ordered by age descending -> Cate(42), Bob(35), Dan(30).
	adults, err := repo.FindAll(ctx, repository.NewQuery().
		Where("age", repository.Ge, 30).
		OrderByDesc("age"))
	if err != nil {
		fail("findAll adults: %v", err)
	}
	if len(adults) != 3 || adults[0].Name != "Cate" || adults[2].Name != "Dan" {
		fail("findAll adults: got %v", names(adults))
	}
	fmt.Printf("composite condition + sort OK: %v\n", names(adults))

	// --- Paging (Total independent of window) ------------------------------
	// Same filter, window the first 2 by age ascending: Dan(30), Bob(35),
	// but Total counts all 3 matches.
	page, err := repo.FindPage(ctx, repository.NewQuery().
		Where("age", repository.Ge, 30).
		OrderBy("age").
		Slice(0, 2))
	if err != nil {
		fail("findPage: %v", err)
	}
	if len(page.Items) != 2 || page.Total != 3 || !page.HasNext() {
		fail("findPage: items=%v total=%d hasNext=%v, want 2/3/true",
			names(page.Items), page.Total, page.HasNext())
	}
	fmt.Printf("paging OK: window=%v total=%d hasNext=%v\n",
		names(page.Items), page.Total, page.HasNext())

	// --- Save refreshes UpdatedAt, keeps CreatedBy -------------------------
	bob := people[1]
	prevUpdated := bob.UpdatedAt
	time.Sleep(5 * time.Millisecond)
	bob.Age = 36
	if err := repo.Save(ctx, bob); err != nil {
		fail("save bob: %v", err)
	}
	reloaded, _, _ := repo.FindByID(ctx, 2)
	if reloaded.Age != 36 || reloaded.CreatedBy != "alice" || !reloaded.UpdatedAt.After(prevUpdated) {
		fail("save: age=%d createdBy=%q updatedAt-advanced=%v",
			reloaded.Age, reloaded.CreatedBy, reloaded.UpdatedAt.After(prevUpdated))
	}
	fmt.Println("save OK: updatedAt refreshed, createdBy preserved")

	// --- Delete -------------------------------------------------------------
	if err := repo.Delete(ctx, 1); err != nil {
		fail("delete(1): %v", err)
	}
	if ok, _ := repo.ExistsByID(ctx, 1); ok {
		fail("delete(1): still exists")
	}
	if n, _ := repo.Count(ctx); n != 3 {
		fail("count after delete: got %d, want 3", n)
	}
	fmt.Println("delete OK: 3 rows remain")

	fmt.Println("ALL OK")
	os.Exit(0)
}

func names(ps []Person) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}
