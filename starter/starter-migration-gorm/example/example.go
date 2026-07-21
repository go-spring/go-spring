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

package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/data/migration"
	"go-spring.org/spring/gs"
	migrationgorm "go-spring.org/starter-migration-gorm"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ----------------------------------------------------------------------------
// This example demonstrates starter-migration-gorm end to end. The application
// supplies two things by name — a *gorm.DB bean "app" and a migration.Source
// bean "app" backed by an embedded migrations/ directory — and blank-imports the
// starter. On startup the starter's Runner (a gs.Runner) applies the pending
// migrations before the goroutine below runs, so widgets already exists and is
// seeded. The test then proves the three headline guarantees: startup apply,
// second-run idempotency, and checksum-drift fail-fast on an edited script.
// ----------------------------------------------------------------------------

//go:embed migrations
var migrationsFS embed.FS

// App holds the migrated database so the smoke test can inspect it. Exporting it
// as gs.Rooter makes it a root bean, so the container wires it and shares the
// same *gorm.DB bean the starter's Runner migrated.
type App struct {
	DB *gorm.DB `autowire:""`
}

func newApp() *App { return &App{} }

// openDB opens a shared-cache in-memory sqlite database on a single connection so
// the migrated schema is visible to every reader of this one *gorm.DB bean.
func openDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file:migration-demo?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	return db, nil
}

func main() {
	// The *gorm.DB bean and the migration.Source bean are both named "app" to
	// match the spring.migration.app config entry; the starter matches DB, source
	// and config entry by that shared name.
	gs.Provide(openDB).Name("app")
	gs.Provide(func() migration.Source {
		return migration.NewFSSource(migrationsFS, "migrations")
	}).Name("app")

	appBean := gs.Provide(newApp).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(appBean.Interface().(*App))
	}()

	gs.Run()
}

// runTest asserts the three guarantees against the already-migrated database and
// exits non-zero on any deviation so check.sh fails.
func runTest(app *App) {
	ctx := context.Background()
	db := app.DB

	// Guarantee 1 — startup apply: the starter's Runner ran both migrations before
	// this goroutine, so schema_migrations records two versions and widgets is
	// seeded with two rows.
	var applied int64
	db.Table("schema_migrations").Count(&applied)
	var widgets int64
	db.Table("widgets").Count(&widgets)
	if applied != 2 || widgets != 2 {
		log.Errorf(ctx, log.TagAppDef, "startup apply: schema_migrations=%d widgets=%d, want 2/2", applied, widgets)
		os.Exit(1)
	}
	fmt.Println("startup apply OK: 2 migrations applied, widgets seeded with 2 rows")

	// Guarantee 2 — idempotency: a second Runner over the same database and the
	// same source applies nothing, because both versions are already recorded.
	store, err := migrationgorm.NewStore(db, "schema_migrations")
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "idempotency: new store: %v", err)
		os.Exit(1)
	}
	src := migration.NewFSSource(migrationsFS, "migrations")
	replayed, err := migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)
	if err != nil || len(replayed) != 0 {
		log.Errorf(ctx, log.TagAppDef, "idempotency: applied %d migration(s), err=%v, want 0/nil", len(replayed), err)
		os.Exit(1)
	}
	fmt.Println("idempotency OK: second run applied 0 migrations")

	// Guarantee 3 — checksum drift: a source whose V1 body differs from the applied
	// one (a hand-edited historical script) is rejected with a checksum error
	// rather than silently re-run or ignored.
	tampered := migration.NewSource(migration.Migration{
		Version:  1,
		Name:     "create widgets",
		Checksum: "0000000000000000000000000000000000000000000000000000000000000000",
		Up: func(ctx context.Context, exec migration.Execer) error {
			return exec.ExecContext(ctx, "SELECT 1")
		},
	})
	if _, err := migration.NewRunner(store, tampered, migration.Options{}).Migrate(ctx); err == nil {
		log.Errorf(ctx, log.TagAppDef, "checksum drift: expected an error, got nil")
		os.Exit(1)
	} else {
		fmt.Println("checksum drift OK: edited historical script rejected -", err.Error())
	}

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory to this source file's directory so the
// relative config path resolves regardless of the launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
