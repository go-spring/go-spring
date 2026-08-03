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

// This example demonstrates the "configtree" configuration provider — the
// sibling of "file-watch" registered by the same starter. Where file-watch
// imports a single configuration document, configtree imports a directory of
// scalar key files: each file's name is a property key and its content is the
// raw value. This is the shape of a Kubernetes Secret / env-style ConfigMap
// mount, reproduced here with the exact atomic ..data symlink swap the kubelet
// performs:
//
//		mount/
//		  ..data              -> ..2025_..._NNN   (symlink, atomically swapped on update)
//		  ..2025_..._NNN/     (timestamped data dir holding the real key files)
//		  db.user             -> ..data/db.user            (symlink, value "alice")
//		  db.password         -> ..data/db.password        (symlink, value "s3cr3t")
//		  server.port         -> ..data/server.port        (symlink, value "8080")
//
//	 1. app.properties imports the tree via spring.app.imports=configtree:./mount
//	 2. A bean binds db.user to a gs.Dync[string] field.
//	 3. The test rewrites the Secret the way the kubelet does — write a new
//	    timestamped dir, then atomically rename ..data onto it. The provider's
//	    directory watcher sees the rename and triggers a property refresh, and
//	    the bound field updates without a restart.
//
// Note: K8s Secret/ConfigMap mounts are flat (one file per key). configtree also
// supports genuinely nested trees (db/user -> property db.user); that case is
// covered by the starter's unit tests.
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

	_ "go-spring.org/starter-config-file"
)

const mountDir = "./mount"

// Demo binds dynamic fields sourced from the watched Secret-style mount.
type Demo struct {
	User     gs.Dync[string] `value:"${db.user:=none}"`
	Password gs.Dync[string] `value:"${db.password:=none}"`
	Port     gs.Dync[string] `value:"${server.port:=none}"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	// Clean up leftover mount from a prior run so the symlink swap always starts
	// from a known-good state.
	_ = os.RemoveAll(mountDir)

	// Lay down the initial Secret-style mount before the app starts so the import
	// resolves at startup.
	if err := writeSecret(map[string]string{
		"db.user":     "alice",
		"db.password": "s3cr3t",
		"server.port": "8080",
	}); err != nil {
		fmt.Fprintln(os.Stderr, "setup mount failed:", err)
		os.Exit(1)
	}

	demo := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(demo.Interface().(*Demo))
		}()
	} else {
		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

func runTest(d *Demo) {
	ctx := context.Background()

	// Every flat key file becomes one property; all three are visible at startup.
	if got := d.User.Value(); got != "alice" {
		log.Errorf(ctx, log.TagAppDef, "unexpected db.user: %q", got)
		os.Exit(1)
	}
	if got := d.Password.Value(); got != "s3cr3t" {
		log.Errorf(ctx, log.TagAppDef, "unexpected db.password: %q", got)
		os.Exit(1)
	}
	if got := d.Port.Value(); got != "8080" {
		log.Errorf(ctx, log.TagAppDef, "unexpected server.port: %q", got)
		os.Exit(1)
	}
	fmt.Printf("initial: db.user=%s server.port=%s\n", d.User.Value(), d.Port.Value())

	// Update the Secret the way Kubernetes does (atomic ..data symlink swap).
	want := "bob-" + time.Now().Format("150405")
	if err := writeSecret(map[string]string{
		"db.user":     want,
		"db.password": "s3cr3t",
		"server.port": "8080",
	}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "update mount failed: %v", err)
		os.Exit(1)
	}

	// The directory watcher observes the swap and triggers a refresh, which
	// re-reads the tree and updates the bound gs.Dync field.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if got := d.User.Value(); got == want {
			fmt.Println("hot-reload observed: db.user=", got)
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Errorf(ctx, log.TagAppDef, "hot-reload timeout: db.user=%q want=%q", d.User.Value(), want)
	os.Exit(1)
}

// writeSecret writes the key/value pairs into the mount using the same atomic
// scheme as the kubelet: the payload goes into a fresh timestamped data
// directory, then the ..data symlink is atomically renamed onto it. Key
// symlinks (db.user -> ..data/db.user, ...) are created once and survive the
// swap because they point through ..data.
func writeSecret(kv map[string]string) error {
	if err := os.MkdirAll(mountDir, 0o755); err != nil {
		return err
	}

	// Fresh timestamped data dir (nanoseconds keep successive writes distinct).
	dataDir := fmt.Sprintf("..%d", time.Now().UnixNano())
	dataPath := filepath.Join(mountDir, dataDir)
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return err
	}
	for key, val := range kv {
		if err := os.WriteFile(filepath.Join(dataPath, key), []byte(val), 0o644); err != nil {
			return err
		}
	}

	// Atomically point ..data at the new data dir via a temp symlink + rename.
	dataLink := filepath.Join(mountDir, "..data")
	tmpLink := filepath.Join(mountDir, "..data_tmp")
	_ = os.Remove(tmpLink)
	if err := os.Symlink(dataDir, tmpLink); err != nil {
		return err
	}
	if err := os.Rename(tmpLink, dataLink); err != nil {
		return err
	}

	// Ensure each key symlink exists (created once; points through ..data).
	for key := range kv {
		keyLink := filepath.Join(mountDir, key)
		if _, err := os.Lstat(keyLink); os.IsNotExist(err) {
			if err := os.Symlink(filepath.Join("..data", key), keyLink); err != nil {
				return err
			}
		}
	}
	return nil
}

// init sets the working directory of the application to the directory where
// this source file resides, so relative file operations are based on the source
// file location, not the process launch path.
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
