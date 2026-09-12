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

// Package main is the self-contained example for starter-gorm-sqlite: it opens
// an in-memory SQLite database, runs a migrate/CRUD/transaction round trip, and
// self-exits on success. No docker needed — SQLite is in-process.
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

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"gorm.io/gorm"

	gormcore "go-spring.org/starter-gorm"
	_ "go-spring.org/starter-gorm-sqlite"
)

type Service struct {
	DB *gormcore.DB `autowire:"sqlite.primary"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {

		// Run the Go-Spring application.

		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

type greeting struct {
	ID      uint   `gorm:"primaryKey"`
	Message string `gorm:"size:255"`
}

// runTest exercises the SQLite round trip: version query, auto-migrate,
// create/read/transaction-update.
func runTest(s *Service) {
	ctx := context.Background()

	var version string
	if err := s.DB.WithContext(ctx).Raw("SELECT sqlite_version()").Scan(&version).Error; err != nil {
		log.Errorf(ctx, log.TagAppDef, "VERSION failed: %v", err)
		os.Exit(1)
	}

	if err := s.DB.WithContext(ctx).AutoMigrate(&greeting{}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "MIGRATE failed: %v", err)
		os.Exit(1)
	}
	g := greeting{Message: "hello"}
	if err := s.DB.WithContext(ctx).Create(&g).Error; err != nil {
		log.Errorf(ctx, log.TagAppDef, "CREATE failed: %v", err)
		os.Exit(1)
	}

	// Transaction update: read then update inside a real gorm transaction.
	var loaded greeting
	if err := s.DB.WithContext(ctx).First(&loaded, 1).Error; err != nil || loaded.Message != "hello" {
		log.Errorf(ctx, log.TagAppDef, "READ failed: msg=%q err=%v", loaded.Message, err)
		os.Exit(1)
	}
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&greeting{}).Where("id = 1").Update("message", "world").Error
	}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "TX failed: %v", err)
		os.Exit(1)
	}
	if err := s.DB.WithContext(ctx).First(&loaded, 1).Error; err != nil || loaded.Message != "world" {
		log.Errorf(ctx, log.TagAppDef, "TX-READ failed: msg=%q err=%v", loaded.Message, err)
		os.Exit(1)
	}

	fmt.Println("SQLite round trip OK: sqlite_version=", version, "final=", loaded.Message)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
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
