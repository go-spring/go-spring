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

package fileutil

import (
	"errors"
	"os"
)

// PathExists reports whether the given path exists.
//
// It returns (false, nil) if the path does not exist.
// If an error other than os.ErrNotExist occurs (for example, a permission
// error), it returns (false, err).
//
// A return value of true indicates that the path exists and was successfully
// stat-ed, but does not distinguish between files and directories.
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ReadDirNames returns the names of all directory entries in the specified
// directory.
//
// The order of the returned names is filesystem-dependent.
// If an error occurs while reading the directory, it may return a non-nil
// slice along with the error.
func ReadDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	names, err := f.Readdirnames(-1)
	return names, err
}
