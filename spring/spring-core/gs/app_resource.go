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
	"io"
	"os"
	"path/filepath"
)

// Resource 具有名字的 io.Reader 接口称为资源。
type Resource interface {
	io.Reader
	Name() string
}

// ResourceLocator 查找名字为 filename 的资源。
type ResourceLocator interface {
	Locate(filename string) ([]Resource, error)
}

// defaultResourceLocator 从本地文件系统中查找资源。
type defaultResourceLocator struct {
	configLocations []string `value:"${spring.config.locations:=config/}"`
}

func (locator *defaultResourceLocator) Locate(filename string) ([]Resource, error) {
	var resources []Resource
	for _, location := range locator.configLocations {
		fileLocation := filepath.Join(location, filename)
		file, err := os.Open(fileLocation)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		resources = append(resources, file)
	}
	return resources, nil
}
