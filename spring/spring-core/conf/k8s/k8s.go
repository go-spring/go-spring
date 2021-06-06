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
	"io/ioutil"
	"net/url"
	"os"

	"github.com/go-spring/spring-core/util"
	"gopkg.in/yaml.v2"
)

// Scheme 从扩展后的 URL 读取文件数据，同时返回文件的扩展名，读取失败时返回错误。
func Scheme(u *url.URL) (_ []byte, ext string, _ error) {

	if u.Host != "" {
		return nil, "", util.UnimplementedMethod
	}

	b, err := ioutil.ReadFile(u.Path)
	if err != nil {
		return nil, "", err
	}

	m := make(map[string]interface{})
	if err = yaml.Unmarshal(b, &m); err != nil {
		return nil, "", err
	}

	data, ok := m["data"]
	if !ok {
		return nil, "", errors.New("data not found")
	}

	if u.Fragment == "" {
		b, err = yaml.Marshal(data)
		return b, ".yaml", err
	}

	d, ok := data.(map[interface{}]interface{})
	if !ok {
		return nil, "", errors.New("data isn't map")
	}

	v, ok := d[u.Fragment]
	if !ok {
		return nil, "", os.ErrNotExist
	}

	s, ok := v.(string)
	if !ok {
		return nil, "", errors.New("node isn't string")
	}

	return []byte(s), ".yaml", nil
}
