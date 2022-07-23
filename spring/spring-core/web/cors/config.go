/*
 * Copyright 2012-2022 the original author or authors.
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

package cors

import (
	"time"
)

type CorsConfig struct {
	AllowOrigins     []string      `value:"${allow-origins:=*}"`
	AllowMethods     []string      `value:"${allow-methods:=*}"`
	AllowHeaders     []string      `value:"${allow-headers:=*}"`
	ExposeHeaders    []string      `value:"${expose-headers:=}"`
	AllowCredentials bool          `value:"${allow-credentials:=false}"`
	MaxAge           time.Duration `value:"${max-age:=0}"`
}

func NewDefaultCorsConfig() CorsConfig {
	c := CorsConfig{}
	c.AllowHeaders = []string{"*"}
	c.AllowOrigins = []string{"*"}
	c.AllowMethods = []string{"*"}
	c.AllowCredentials = false
	c.MaxAge = time.Duration(0)

	return c
}
