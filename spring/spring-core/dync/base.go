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

package dync

import (
	"fmt"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gsl"
)

type Base struct {
	param conf.BindParam
}

func (v *Base) GetParam() conf.BindParam {
	return v.param
}

func (v *Base) setParam(param conf.BindParam) {
	v.param = param
}

func (v *Base) Property(prop *conf.Properties) (string, error) {
	key := v.param.Key
	if !prop.Has(key) && !v.param.Tag.HasDef {
		return "", fmt.Errorf("property %q not exist", key)
	}
	s := prop.Get(key, conf.Def(v.param.Tag.Def))
	return s, nil
}

func (v *Base) Validate(val string) error {
	if v.param.Validate == "" {
		return nil
	}
	b, err := gsl.Eval(v.param.Validate, val)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("validate failed on %q for value %s", v.param.Validate, val)
	}
	return nil
}
