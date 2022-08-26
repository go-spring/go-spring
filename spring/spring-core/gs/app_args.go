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
	"errors"
	"strings"

	"github.com/go-spring/spring-core/conf"
)

// LoadCmdArgs 加载以 -D key=value 或者 -D key[=true] 形式传入的命令行参数。
func LoadCmdArgs(args []string, p *conf.Properties) error {
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s == "-D" {
			if i >= len(args)-1 {
				return errors.New("cmd option -D needs arg")
			}
			next := args[i+1]
			ss := strings.SplitN(next, "=", 2)
			if len(ss) == 1 {
				ss = append(ss, "true")
			}
			if err := p.Set(ss[0], ss[1]); err != nil {
				return err
			}
		}
	}
	return nil
}
