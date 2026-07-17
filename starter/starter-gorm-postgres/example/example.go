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

	_ "go-spring.org/starter-gorm-postgres"
)

// KV is a small demo model. `key` and `value` are reserved words in some SQL dialects,
// so we map the columns to `kkey` / `vvalue` via GORM tags.
type KV struct {
	ID    uint   `gorm:"primaryKey"`
	Key   string `gorm:"column:kkey;size:64;uniqueIndex"`
	Value string `gorm:"column:vvalue;size:255"`
}

type Service struct {
	DB *gorm.DB `autowire:"primary"`
}

func main() {

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/postgres_version", func(w http.ResponseWriter, r *http.Request) {
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
	// ~ curl http://127.0.0.1:9090/postgres_version
	// PostgreSQL 16.x ...
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
	if err := s.DB.AutoMigrate(&KV{}); err != nil {
		fmt.Fprintln(os.Stderr, "auto migrate failed:", err)
		os.Exit(1)
	}
	if !s.DB.Migrator().HasTable(&KV{}) {
		fmt.Fprintln(os.Stderr, "table not created")
		os.Exit(1)
	}

	// Feature 2: Create + First (basic CRUD).
	if err := s.DB.Create(&KV{Key: "key", Value: "value"}).Error; err != nil {
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

	// Feature 3: Transaction — update the row inside a transaction.
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		return tx.Model(&KV{}).Where("kkey = ?", "key").
			Update("vvalue", "value2").Error
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "transaction failed:", err)
		os.Exit(1)
	}
	var updated KV
	if err := s.DB.First(&updated, "kkey = ?", "key").Error; err != nil {
		fmt.Fprintln(os.Stderr, "first (post-tx) failed:", err)
		os.Exit(1)
	}
	if updated.Value != "value2" {
		fmt.Fprintln(os.Stderr, "unexpected value after tx:", updated.Value)
		os.Exit(1)
	}

	fmt.Println("Response from server:", version, "kv:", updated.Value)
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
