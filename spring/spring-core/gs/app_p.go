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

package gs

import (
	"os"
	"path/filepath"
)

type propertySource struct {
	file   string
	prefix string
	object interface{}
}

type ResourceLocator interface {
	Locate(filename string) ([]*os.File, error)
}

type defaultResourceLocator struct {
	configLocations []string `value:"${spring.config.locations:=config/}"`
}

func (ps *defaultResourceLocator) Locate(filename string) ([]*os.File, error) {
	var files []*os.File
	for _, location := range ps.configLocations {
		fileLocation := filepath.Join(location, filename)
		file, err := os.Open(fileLocation)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}
