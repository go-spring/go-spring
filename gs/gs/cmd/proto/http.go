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

package proto

import (
	"os"
	"os/exec"
	"path/filepath"

	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
)

// GenHttp regenerates the HTTP server code under `<currDir>/idl/http/proto`
// by invoking the external `gs-http-gen` binary.
//
// The output directory is wiped and recreated so stale artifacts from a
// previous run never leak into the new generation.
func GenHttp(currDir string) error {
	dir := filepath.Join(currDir, "idl/http/proto")
	if err := os.RemoveAll(dir); err != nil {
		return errutil.Explain(err, "clear output dir %s", dir)
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errutil.Explain(err, "create output dir %s", dir)
	}

	cmd := exec.Command("gs-http-gen", "--server", "--output", dir)
	cmd.Dir = filepath.Dir(dir)
	return runcmd.Run(cmd, "Generating HTTP server code")
}
