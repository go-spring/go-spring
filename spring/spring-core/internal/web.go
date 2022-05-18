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

package internal

// WebServerConfig Web 服务器配置，通常搭配 Prefix (路由前缀)一起使用。
type WebServerConfig struct {
	Host         string `value:"${host:=}"`            // 监听 IP
	Port         int    `value:"${port:=8080}"`        // HTTP 端口
	EnableSSL    bool   `value:"${ssl.enable:=false}"` // 是否启用 HTTPS
	KeyFile      string `value:"${ssl.key:=}"`         // SSL 秘钥
	CertFile     string `value:"${ssl.cert:=}"`        // SSL 证书
	BasePath     string `value:"${base-path:=}"`       // 根路径
	Prefix       string `value:"${prefix:=}"`          // 路由前缀
	ReadTimeout  int    `value:"${read-timeout:=0}"`   // 读取超时，毫秒
	WriteTimeout int    `value:"${write-timeout:=0}"`  // 写入超时，毫秒
}
