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
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// GenHttp generates HTTP code by invoking an external generator ("gs-http-gen").
func GenHttp(currDir string) {

	dir := filepath.Join(currDir, "idl/http/proto")
	if err := os.RemoveAll(dir); err != nil {
		log.Fatalln(err)
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Fatalln(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		f := bufio.NewReader(r)
		for {
			line, _, err := f.ReadLine()
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Fatalln(err)
			}
			fmt.Print(string(line))
		}

		// Close read-end of pipe after reading is done
		_ = r.Close()
	}()

	cmd := exec.Command("gs-http-gen", "--server", "--output", dir)
	cmd.Dir = filepath.Dir(dir)
	cmd.Stdout = w
	cmd.Stderr = w
	if err = cmd.Run(); err != nil {
		log.Fatalln(err)
	}

	// Close the write-end of the pipe after the command finishes
	_ = w.Close()
}
