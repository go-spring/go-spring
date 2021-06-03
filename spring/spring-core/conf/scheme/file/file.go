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

package file

import (
	"github.com/go-spring/spring-core/conf/fs"
	"github.com/go-spring/spring-core/conf/scheme"
)

type Scheme struct {
	fs fs.FS
}

func New(fs fs.FS) scheme.Scheme {
	return &Scheme{fs: fs}
}

func (s *Scheme) Split(path string) (location, filename string) {
	return s.fs.Split(path)
}

func (s *Scheme) Open(location string) (scheme.Reader, error) {
	return &reader{fs: s.fs, location: location}, nil
}

type reader struct {
	fs       fs.FS
	location string
}

func (r *reader) ReadFile(filename string) (b []byte, ext string, err error) {
	return r.fs.ReadFile(r.fs.Join(r.location, filename))
}
