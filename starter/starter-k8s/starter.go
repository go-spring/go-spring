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

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs"
	"github.com/spf13/viper"
)

func init() {
	gs.Bootstrap().Object(new(propertySource)).Export((*gs.PropertySource)(nil))
}

type propertySource struct {
	ConfigLocation string `value:"${spring.cloud.k8s.config-location:=config/config-map.yml}"`
}

func (ps *propertySource) Load(e gs.Environment) (map[string]*conf.Properties, error) {

	var err error

	v := viper.New()
	v.SetConfigFile(ps.ConfigLocation)
	if err = v.ReadInConfig(); err != nil {
		return nil, err
	}

	d := v.Sub("data")
	if d == nil {
		return nil, fmt.Errorf("data not found in %s", ps.ConfigLocation)
	}

	result := make(map[string]*conf.Properties)
	if result[""], err = ps.loadProfile(e, d, "application"); err != nil {
		return nil, err
	}
	if profile := e.ActiveProfile(); profile != "" {
		result[profile], err = ps.loadProfile(e, d, "application-"+profile)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (ps *propertySource) loadProfile(e gs.Environment, v *viper.Viper, filename string) (*conf.Properties, error) {
	r := conf.New()
	for _, ext := range e.ConfigExtensions() {
		key := filename + ext
		if !v.IsSet(key) {
			continue
		}
		val := v.GetString(key)
		p, err := conf.Read([]byte(val), ext)
		if err != nil {
			return nil, err
		}
		for _, s := range p.Keys() {
			r.Set(s, p.Get(s))
		}
	}
	return r, nil
}
