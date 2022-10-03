/*
 * Copyright 2012-2019 the original author or authors.
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

package util

import (
	"os"
)

// ReadDirNames reads the directory named by dirname and returns an unsorted
// list of directory entries.
func ReadDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	return names, err
}

// Contract contracts `filename` and replace the excessive part using `...`.
func Contract(filename string, maxLength int) string {
	if n := len(filename); maxLength > 3 && n > maxLength-3 {
		return "..." + filename[n-maxLength+3:]
	}
	return filename
}
