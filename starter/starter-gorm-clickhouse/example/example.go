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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/spring/gs"
	"gorm.io/gorm"

	_ "go-spring.org/starter-gorm-clickhouse"
)

// KV is a small demo model. `key` and `value` can be reserved-ish words, so we
// map columns to `kkey` / `vvalue`. ClickHouse does NOT enforce unique indexes
// the way OLTP engines do, so we deliberately do not use `uniqueIndex` here.
type KV struct {
	ID    uint64 `gorm:"primaryKey"`
	Key   string `gorm:"column:kkey"`
	Value string `gorm:"column:vvalue"`
}

// TableName pins the table name; combined with the MergeTree table option below
// this keeps AutoMigrate deterministic against a plain ClickHouse server.
func (KV) TableName() string { return "kv" }

type Service struct {
	DB          *gorm.DB `autowire:"primary"`
	DiscoveryDB *gorm.DB `autowire:"discovery"`
}

func main() {

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/clickhouse_version", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		var version string
		err := s.DB.Raw("SELECT version()").Scan(&version).Error
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(version))
	})

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage:
	//
	// ~ curl http://127.0.0.1:9090/clickhouse_version
	// 24.x.x.x%
}

func runTest(s *Service) {
	var version string
	if err := s.DB.Raw("SELECT version()").Scan(&version).Error; err != nil {
		fmt.Fprintln(os.Stderr, "query failed:", err)
		os.Exit(1)
	}
	if version == "" {
		fmt.Fprintln(os.Stderr, "empty version")
		os.Exit(1)
	}

	// Ensure the test is deterministic/idempotent across repeated smoke runs.
	if err := s.DB.Migrator().DropTable(&KV{}); err != nil {
		fmt.Fprintln(os.Stderr, "drop table failed:", err)
		os.Exit(1)
	}

	// Feature 1: AutoMigrate — create the table from the model.
	// ClickHouse requires an engine; supply MergeTree + ORDER BY via table options.
	if err := s.DB.Set("gorm:table_options", "ENGINE=MergeTree ORDER BY (id)").
		AutoMigrate(&KV{}); err != nil {
		fmt.Fprintln(os.Stderr, "auto migrate failed:", err)
		os.Exit(1)
	}
	if !s.DB.Migrator().HasTable(&KV{}) {
		fmt.Fprintln(os.Stderr, "table not created")
		os.Exit(1)
	}

	// Feature 2: Create + First (basic CRUD).
	if err := s.DB.Create(&KV{ID: 1, Key: "key", Value: "value"}).Error; err != nil {
		fmt.Fprintln(os.Stderr, "create failed:", err)
		os.Exit(1)
	}
	var got KV
	if err := s.DB.First(&got, "kkey = ?", "key").Error; err != nil {
		fmt.Fprintln(os.Stderr, "first failed:", err)
		os.Exit(1)
	}
	if got.Value != "value" {
		fmt.Fprintln(os.Stderr, "unexpected value after create:", got.Value)
		os.Exit(1)
	}

	// Feature 3: Batch Create + Count.
	// ClickHouse has no standard multi-statement transactions, so instead of
	// s.DB.Transaction(...) we demonstrate a batch insert and a Count.
	batch := []KV{
		{ID: 2, Key: "k2", Value: "v2"},
		{ID: 3, Key: "k3", Value: "v3"},
	}
	if err := s.DB.Create(&batch).Error; err != nil {
		fmt.Fprintln(os.Stderr, "batch create failed:", err)
		os.Exit(1)
	}
	var count int64
	if err := s.DB.Model(&KV{}).Count(&count).Error; err != nil {
		fmt.Fprintln(os.Stderr, "count failed:", err)
		os.Exit(1)
	}
	if count < 3 {
		fmt.Fprintln(os.Stderr, "unexpected row count:", count)
		os.Exit(1)
	}

	fmt.Println("Response from server:", version, "count:", count)

	// Feature 4: the discovery-backed client. Its address came from the
	// registered discovery backend (service-name=clickhouse-cluster), not from
	// conf's dummy addr, so a successful round-trip proves discovery is wired.
	var discVersion string
	if err := s.DiscoveryDB.Raw("SELECT version()").Scan(&discVersion).Error; err != nil {
		fmt.Fprintln(os.Stderr, "discovery query failed:", err)
		os.Exit(1)
	}
	if discVersion == "" {
		fmt.Fprintln(os.Stderr, "discovery empty version")
		os.Exit(1)
	}
	fmt.Println("Response from discovered server:", discVersion)

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
