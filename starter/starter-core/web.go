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

package StarterCore

import (
	"time"
)

// WebServerConfig Web 服务器配置
type WebServerConfig struct {
	IP           string `value:"${web.server.ip:=}"`              // 监听 IP
	Port         int    `value:"${web.server.port:=8080}"`        // HTTP 端口
	EnableSSL    bool   `value:"${web.server.ssl.enable:=false}"` // 是否启用 HTTPS
	KeyFile      string `value:"${web.server.ssl.key:=}"`         // SSL 秘钥
	CertFile     string `value:"${web.server.ssl.cert:=}"`        // SSL 证书
	BasePath     string `value:"${web.server.base-path:=/}"`      // 根路径
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}
