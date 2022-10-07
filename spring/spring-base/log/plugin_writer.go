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

package log

import (
	"context"
	"io"
	"os"
	"sync"
)

// Writers manages the Get and Release of Writer(s).
var Writers = &writers{
	writers: make(map[string]*sharedWriter),
}

// Writer is io.Writer with a name and a Stop method.
type Writer interface {
	io.Writer
	Name() string
	Stop(ctx context.Context)
}

// writers manages the Get and Release of Writer(s).
type writers struct {
	lock    sync.Mutex
	writers map[string]*sharedWriter
}

// sharedWriter wrappers count when decreases to 0 the Writer will be released.
type sharedWriter struct {
	writer Writer
	count  int32
}

// Has returns true if a Writer named by name is cached, otherwise false.
func (s *writers) Has(name string) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, ok := s.writers[name]
	return ok
}

// Get returns a Writer that created by fn and named by name.
func (s *writers) Get(name string, fn func() (Writer, error)) (Writer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	sw, ok := s.writers[name]
	if ok {
		sw.count++
		return sw.writer, nil
	}
	w, err := fn()
	if err != nil {
		return nil, err
	}
	sw = &sharedWriter{writer: w}
	s.writers[name] = sw
	sw.count++
	return sw.writer, nil
}

// Release removes a Writer when its share count decreases to 0.
func (s *writers) Release(ctx context.Context, writer Writer) {
	s.lock.Lock()
	defer s.lock.Unlock()
	sw, ok := s.writers[writer.Name()]
	if !ok {
		return
	}
	sw.count--
	if sw.count > 0 {
		return
	}
	delete(s.writers, writer.Name())
	writer.Stop(ctx)
}

// FileWriter is a Writer implementation by *os.File.
type FileWriter struct {
	file *os.File
}

// NewFileWriter returns a FileWriter that a Writer implementation.
func NewFileWriter(fileName string) (Writer, error) {
	flag := os.O_RDWR | os.O_CREATE | os.O_APPEND
	file, err := os.OpenFile(fileName, flag, 666)
	if err != nil {
		return nil, err
	}
	return &FileWriter{file: file}, nil
}

func (c *FileWriter) Name() string {
	return c.file.Name()
}

func (c *FileWriter) Write(p []byte) (n int, err error) {
	return c.file.Write(p)
}

func (c *FileWriter) Stop(ctx context.Context) {

}
