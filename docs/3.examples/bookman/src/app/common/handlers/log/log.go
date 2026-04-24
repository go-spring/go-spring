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

package log

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Module(nil, func(p conf.Properties) error {

		var loggers map[string]struct {
			Name string `value:"${name}"` // Log file name
			Dir  string `value:"${dir}"`  // Directory where the log file will be stored
		}

		// Bind configuration from the "${log}" node into the 'loggers' map.
		err := p.Bind(&loggers, "${log}")
		if err != nil {
			return err
		}

		for k, l := range loggers {
			var (
				f    *os.File
				flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
			)

			// Open (or create) the log file
			f, err = os.OpenFile(filepath.Join(l.Dir, l.Name), flag, os.ModePerm)
			if err != nil {
				return err
			}

			// Create a new slog.Logger instance with a text handler writing to the file
			o := slog.New(slog.NewTextHandler(f, nil))

			// Wrap the logger into a Bean with a destroy hook to close the file
			gs.Object(o).Name(k).Destroy(func(_ *slog.Logger) {
				_ = f.Close()
			})
		}
		return nil
	})
}
