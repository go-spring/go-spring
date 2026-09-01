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

// Command example demonstrates starter-migration-goose end to end. The
// application supplies a *gorm.DB bean "app" and blank-imports the starter; on
// startup the starter's Runner (a gs.Runner) applies the goose SQL migrations
// in ./sql before the goroutine below runs, so widgets already exists and is
// seeded. The test then proves two guarantees: startup apply and second-run
// idempotency.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-migration-goose"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// App holds the migrated database so the smoke test can inspect it. Exporting
// it as gs.Rooter makes it a root bean, so the container wires it and shares
// the same *gorm.DB bean the starter's Runner migrated.
type App struct {
	DB *gorm.DB `autowire:""`
}

func newApp() *App { return &App{} }

// openDB opens a shared-cache in-memory sqlite database on a single connection
// so the migrated schema is visible to every reader of this one *gorm.DB bean.
func openDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file:goose-demo?mode=memory&cache=shared"), &gorm.Config{})
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

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	gs.Provide(openDB).Name("app")
	appBean := gs.Provide(newApp).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(appBean.Interface().(*App))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
	}
	gs.Run()
}

// runTest asserts the guarantees against the already-migrated database and
// exits non-zero on any deviation so check.sh fails.
func runTest(app *App) {
	ctx := context.Background()
	db := app.DB

	// Guarantee 1 — startup apply: the starter's Runner ran both migrations
	// before this goroutine, so goose_db_version records two versions and
	// widgets is seeded with two rows.
	var applied, widgets int64
	db.Table("goose_db_version").Count(&applied)
	db.Table("widgets").Count(&widgets)
	if applied < 2 || widgets != 2 {
		log.Errorf(ctx, log.TagAppDef, "startup apply: goose_db_version=%d widgets=%d, want 2/2", applied, widgets)
		os.Exit(1)
	}
	fmt.Println("startup apply OK: 2 migrations applied, widgets seeded with 2 rows")

	// Guarantee 2 — idempotency: a second goose provider over the same database
	// and the same directory applies nothing.
	sqlDB, _ := db.DB()
	provider, err := goose.NewProvider("", sqlDB, os.DirFS("./sql"),
		goose.WithStore(mustStore(goose.DialectSQLite3)))
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "idempotency: provider: %v", err)
		os.Exit(1)
	}
	res, err := provider.Up(ctx)
	if err != nil || len(res) != 0 {
		log.Errorf(ctx, log.TagAppDef, "idempotency: applied %d migration(s), err=%v, want 0/nil", len(res), err)
		os.Exit(1)
	}
	fmt.Println("idempotency OK: second run applied 0 migrations")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

func mustStore(d goose.Dialect) database.Store {
	store, err := database.NewStore(database.Dialect(d), "goose_db_version")
	if err != nil {
		panic(err)
	}
	return store
}

// init sets the working directory to this source file's directory so the
// relative config path and ./sql resolve regardless of the launch path.
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
