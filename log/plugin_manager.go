/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * You may not use this file except in compliance with the License.
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
	"os"
	"path/filepath"
	"sync"
)

// File is a reference-counted wrapper around *os.File.
// The underlying file must not be used directly.
// Use Write to perform writes.
type File struct {
	name  string
	file  *os.File
	count int
}

// Name returns the name of the file.
func (f *File) Name() string {
	return f.name
}

// Write writes to the file.
func (f *File) Write(p []byte) (int, error) {
	return f.file.Write(p)
}

var fileManager = struct {
	files map[string]*File
	mutex sync.Mutex
}{
	files: make(map[string]*File),
}

// OpenFile returns a shared File for the given name.
// If the file is already open, its reference count is increased.
// Otherwise, the file is opened and tracked.
func OpenFile(name string) (*File, error) {
	name, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}

	fileManager.mutex.Lock()
	defer fileManager.mutex.Unlock()

	if v, ok := fileManager.files[name]; ok {
		v.count++
		return v, nil
	}

	const fileFlag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	f, err := os.OpenFile(name, fileFlag, 0644)
	if err != nil {
		return nil, err
	}

	v := &File{name: name, count: 1, file: f}
	fileManager.files[name] = v
	return v, nil
}

// CloseFile decrements the reference count of f.
// When the count reaches zero, the file is closed.
func CloseFile(f *File) {
	fileManager.mutex.Lock()
	defer fileManager.mutex.Unlock()

	v, ok := fileManager.files[f.name]
	if !ok {
		return
	}

	v.count--
	if v.count > 0 {
		return
	}

	delete(fileManager.files, f.name)
	_ = v.file.Close()
}
