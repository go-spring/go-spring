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

package StarterK8S

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-spring/spring-core/gs"
	"github.com/spf13/viper"
)

func init() {
	gs.Bootstrap().ResourceLocator(new(resourceLocator))
}

type resourceLocator struct {
	ConfigLocation string `value:"${spring.cloud.k8s.config-location:=config/config-map.yml}"`
	tempDir        string
}

func (r *resourceLocator) OnInit(e gs.Environment) error {

	v := viper.New()
	v.SetConfigFile(r.ConfigLocation)
	if err := v.ReadInConfig(); err != nil {
		return err
	}

	d := v.Sub("data")
	if d == nil {
		return fmt.Errorf("data not found in %s", r.ConfigLocation)
	}

	tempDir, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		return err
	}
	r.tempDir = tempDir

	for _, key := range d.AllKeys() {
		val := d.GetString(key)
		filename := filepath.Join(tempDir, key)
		err = ioutil.WriteFile(filename, []byte(val), os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *resourceLocator) Locate(filename string) ([]gs.Resource, error) {
	file, err := os.Open(filepath.Join(r.tempDir, filename))
	if err != nil {
		return nil, err
	}
	return []gs.Resource{file}, nil
}
