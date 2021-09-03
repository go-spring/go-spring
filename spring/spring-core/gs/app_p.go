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

	"github.com/go-spring/spring-core/conf"
)

type propertySource struct {
	file   string
	prefix string
	object interface{}
}

type PropertySourceLocator interface {
	Load(filename string) ([]Properties, error)
}

type filePropertySourceLocator struct {
	configLocations []string `value:"${spring.config.locations:=config/}"`
}

func (ps *filePropertySourceLocator) Load(filename string) ([]Properties, error) {
	var arr []Properties
	for _, location := range ps.configLocations {
		fileLocation := filepath.Join(location, filename)
		p, err := conf.Load(fileLocation)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		arr = append(arr, &FileProperties{p, fileLocation})
	}
	return arr, nil
}
