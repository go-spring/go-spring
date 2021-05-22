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
	"fmt"
	"io/ioutil"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/conf/yaml"
)

func Scheme(p *conf.Properties, fileLocation string, fileName string, configTypes []string) error {

	b, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		return err
	}

	m, err := yaml.Read(b)
	if err != nil {
		return err
	}

	d, ok := m["data"]
	if !ok {
		return fmt.Errorf("data not found in config-map %s", fileLocation)
	}

	data := d.(map[string]interface{})

	for _, configType := range configTypes {
		key := fileName + "." + configType

		v, ok := data[key]
		if !ok {
			continue
		}

		err = p.Read([]byte(v.(string)), configType)
		if err != nil {
			return err
		}
	}
	return nil
}
