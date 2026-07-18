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

// This example demonstrates the "file-watch" configuration provider and the
// file-change -> bean hot-reload link, reproducing the exact layout Kubernetes
// uses when it mounts a ConfigMap/Secret as a volume:
//
//		mount/
//		  ..data            -> ..2025_..._NNN   (symlink, atomically swapped on update)
//		  ..2025_..._NNN/   (timestamped data dir holding the real files)
//		  application.properties -> ..data/application.properties  (symlink)
//
//	 1. app.properties imports config from the mount via
//	    spring.app.imports=file-watch:./mount?format=properties
//	 2. A bean binds demo.message to a gs.Dync[string] field.
//	 3. The test rewrites the ConfigMap the way the kubelet does — write a new
//	    timestamped dir, then atomically rename ..data onto it. The provider's
//	    directory watcher sees the rename and triggers a property refresh, and
//	    the bound field updates without a restart.
package main

import (
	"context"
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

// Demo binds a dynamic configuration field sourced from the watched mount. It
// is registered as a root object so the container creates it eagerly.
type Demo struct {
	Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func main() {
	// Unset env vars that leak from the developer shell so runs are reproducible
	// and consistent with sibling starter examples.
	_ = os.Unsetenv("_")
	_ = os.Unsetenv("TERM")
	_ = os.Unsetenv("TERM_SESSION_ID")

	// Lay down the initial ConfigMap-style mount before the app starts so the
	// import resolves at startup.
	if err := writeConfigMap("demo.message=initial\n"); err != nil {
		fmt.Fprintln(os.Stderr, "setup mount failed:", err)
		os.Exit(1)
	}

	demoBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(demoBean.Interface().(*Demo))
	}()

	gs.Run()
}

func runTest(d *Demo) {
	ctx := context.Background()

	// The initial mounted value is visible at startup.
	if got := d.Message.Value(); got != "initial" {
		log.Errorf(ctx, log.TagAppDef, "unexpected initial value: %q", got)
		os.Exit(1)
	}
	fmt.Println("initial value:", d.Message.Value())

	// Update the ConfigMap the way Kubernetes does (atomic ..data symlink swap).
	want := "updated-" + time.Now().Format("150405")
	if err := writeConfigMap("demo.message=" + want + "\n"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "update mount failed: %v", err)
		os.Exit(1)
	}

	// The directory watcher observes the swap and triggers a refresh, which
	// re-reads the mount and updates the bound gs.Dync field. Poll until the new
	// value is visible or time out.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if got := d.Message.Value(); got == want {
			fmt.Println("hot-reload observed:", got)
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Errorf(ctx, log.TagAppDef, "hot-reload timeout: message=%q want=%q", d.Message.Value(), want)
	os.Exit(1)
}

// writeConfigMap writes application.properties into the mount using the same
// atomic scheme as the kubelet: the payload goes into a fresh timestamped data
// directory, then the ..data symlink is atomically renamed onto it. Key
// symlinks (application.properties -> ..data/application.properties) are created
// once and survive the swap because they point through ..data.
func writeConfigMap(propsContent string) error {
	const key = "application.properties"

	if err := os.MkdirAll(mountDir, 0o755); err != nil {
		return err
	}

	// Fresh timestamped data dir (nanoseconds keep successive writes distinct).
	dataDir := fmt.Sprintf("..%d", time.Now().UnixNano())
	dataPath := filepath.Join(mountDir, dataDir)
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dataPath, key), []byte(propsContent), 0o644); err != nil {
		return err
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

	// Ensure the key symlink exists (created once; points through ..data).
	keyLink := filepath.Join(mountDir, key)
	if _, err := os.Lstat(keyLink); os.IsNotExist(err) {
		if err := os.Symlink(filepath.Join("..data", key), keyLink); err != nil {
			return err
		}
	}
	return nil
}

// init sets the working directory to this source file's directory so relative
// config paths resolve correctly, then clears any mount left by a prior run.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine source file path")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
	_ = os.RemoveAll(mountDir)
}
