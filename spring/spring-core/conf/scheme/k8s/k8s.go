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

package k8s

import (
	"errors"
	"os"
	"strings"

	"github.com/go-spring/spring-core/conf/fs"
	"github.com/go-spring/spring-core/conf/parser/yaml"
	"github.com/go-spring/spring-core/conf/scheme"
)

type Scheme struct{}

func New() scheme.Scheme {
	return &Scheme{}
}

func (_ *Scheme) Split(path string) (location, filename string) {
	if i := strings.LastIndexByte(path, '#'); i > 0 {
		return path[:i], path[i+1:]
	}
	return path, ""
}

func (_ *Scheme) Open(fs fs.FS, location string) (scheme.Reader, error) {

	b, err := fs.ReadFile(location)
	if err != nil {
		return nil, err
	}

	m, err := yaml.New().Parse(b)
	if err != nil {
		return nil, err
	}

	d, ok := m["data"]
	if !ok {
		return nil, errors.New("data not found")
	}

	data, ok := d.(map[string]interface{})
	if !ok {
		return nil, errors.New("data isn't map")
	}

	return &reader{data: data}, nil
}

type reader struct {
	data map[string]interface{}
}

func (r *reader) ReadFile(filename string) ([]byte, error) {

	v, ok := r.data[filename]
	if !ok {
		return nil, os.ErrNotExist
	}

	str, ok := v.(string)
	if !ok {
		return nil, errors.New("error data")
	}

	return []byte(str), nil
}
