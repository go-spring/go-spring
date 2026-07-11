/*
 * Copyright 2025 The Go-Spring Authors.
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

package config

import "github.com/zeromicro/go-zero/rest"

// Config holds the service settings.
//
// In a stock go-zero project these fields come from etc/greet-api.yaml via
// conf.MustLoad. Here they are bound from Go-Spring properties (see
// conf/app.properties) using `value` tags, so the whole flag + YAML loading
// step in main() disappears.
type Config struct {
	Name string `value:"${name:=greet-api}"`
	Host string `value:"${host:=0.0.0.0}"`
	Port int    `value:"${port:=8888}"`
}

// RestConf adapts the Go-Spring-bound Config into the rest.RestConf that
// go-zero's rest.MustNewServer expects. Only the fields the demo needs are
// set; everything else falls back to go-zero's zero-value behaviour
// (console JSON logging, no read timeout, unlimited body size).
func (c Config) RestConf() rest.RestConf {
	var rc rest.RestConf
	rc.Name = c.Name
	rc.Host = c.Host
	rc.Port = c.Port
	return rc
}
