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

package conf

import (
	"time"
)

// WebServerConfig Web 服务器配置。
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

func DefaultWebServerConfig() WebServerConfig {
	return WebServerConfig{
		Port:     8080,
		BasePath: "/",
	}
}

// DatabaseClientConfig 关系型数据库客户端配置。
type DatabaseClientConfig struct {
	Url string `value:"${db.url}"`
}

// RedisClientConfig Redis 客户端配置。
type RedisClientConfig struct {
	Host           string `value:"${redis.host:=127.0.0.1}"`   // IP
	Port           int    `value:"${redis.port:=6379}"`        // 端口号
	Username       string `value:"${redis.username:=}"`        // 用户名
	Password       string `value:"${redis.password:=}"`        // 密码
	Database       int    `value:"${redis.database:=0}"`       // DB 序号
	Ping           bool   `value:"${redis.ping:=true}"`        // 是否 PING 探测
	ConnectTimeout int    `value:"${redis.connect-timeout:=}"` // 连接超时，毫秒
	ReadTimeout    int    `value:"${redis.read-timeout:=}"`    // 读取超时，毫秒
	WriteTimeout   int    `value:"${redis.write-timeout:=}"`   // 写入超时，毫秒
	IdleTimeout    int    `value:"${redis.idle-timeout:=}"`    // 空闲连接超时，毫秒
}

func DefaultRedisClientConfig() RedisClientConfig {
	return RedisClientConfig{
		Host: "127.0.0.1",
		Port: 6379,
		Ping: true,
	}
}

// MongoClientConfig MongoDB 客户端配置。
type MongoClientConfig struct {
	Url string `value:"${mongo.url:=mongodb://localhost}"`
}

// GrpcServerConfig gRPC 服务器配置。
type GrpcServerConfig struct {
	Port int `value:"${grpc.server.port:=9090}"`
}

// GrpcEndpointConfig gRPC 服务端点配置。
type GrpcEndpointConfig struct {
	Address string `value:"${address:=127.0.0.1:9090}"`
}
