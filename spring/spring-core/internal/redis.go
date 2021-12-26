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

// RedisClientConfig Redis 客户端配置，通常配合 redis 服务器名称前缀一起使用。
type RedisClientConfig struct {
	Host           string `value:"${host:=127.0.0.1}"`    // IP
	Port           int    `value:"${port:=6379}"`         // 端口号
	Username       string `value:"${username:=}"`         // 用户名
	Password       string `value:"${password:=}"`         // 密码
	Database       int    `value:"${database:=0}"`        // DB 序号
	Ping           bool   `value:"${ping:=true}"`         // 是否 PING 探测
	ConnectTimeout int    `value:"${connect-timeout:=0}"` // 连接超时，毫秒
	ReadTimeout    int    `value:"${read-timeout:=0}"`    // 读取超时，毫秒
	WriteTimeout   int    `value:"${write-timeout:=0}"`   // 写入超时，毫秒
	IdleTimeout    int    `value:"${idle-timeout:=0}"`    // 空闲连接超时，毫秒
}
