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

	"github.com/go-spring/spring-base/atomic"
)

var (
	writers sync.Map
)

type Writer interface {
	io.Writer
	Name() string
	Stop(ctx context.Context)
}

type result struct {
	wg  sync.WaitGroup
	cnt atomic.Int32
	w   Writer
	err error
}

func NewWriter(name string, fn func() (Writer, error)) (Writer, error) {
	c := &result{}
	c.wg.Add(1)
	actual, loaded := writers.LoadOrStore(name, c)
	if loaded {
		c = actual.(*result)
		c.wg.Wait()
		if c.err != nil {
			return nil, c.err
		}
		c.cnt.Add(1)
		return c.w, nil
	}
	c.w, c.err = fn()
	c.wg.Done()
	if c.err != nil {
		writers.Delete(name)
		return nil, c.err
	}
	c.cnt.Add(1)
	return c.w, nil
}

func DestroyWriter(w Writer) {
	v, ok := writers.Load(w.Name())
	if !ok {
		return
	}
	c := v.(*result)
	if c.w != w {
		return
	}
	n := c.cnt.Add(-1)
	if n > 0 {
		return
	}
	writers.Delete(w.Name())
}

type FileWriter struct {
	file *os.File
}

func NewFileWriter(fileName string) (*FileWriter, error) {
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
