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

// Package fastdev 流量录制和回放，名称源自 didi fastdev 项目。
package fastdev

import (
	"encoding/hex"

	"github.com/google/uuid"
)

// RecordSessionIDKey 流量录制模式下存储会话 ID 使用的 Key 。
const RecordSessionIDKey = "RECORD-SESSION-ID"

// ReplaySessionIDKey 流量回放模式下存储会话 ID 使用的 Key 。
const ReplaySessionIDKey = "REPLAY-SESSION-ID"

// NewSessionID 使用 uuid 算法生成新的 Session ID 。
func NewSessionID() string {
	u := uuid.New()
	buf := make([]byte, 32)
	hex.Encode(buf, u[:4])
	hex.Encode(buf[8:12], u[4:6])
	hex.Encode(buf[12:16], u[6:8])
	hex.Encode(buf[16:20], u[8:10])
	hex.Encode(buf[20:], u[10:])
	return string(buf)
}

const (
	HTTP  = "http"
	REDIS = "redis"
	APCU  = "apcu"
)

// Action 将上下游调用、缓存获取、文件写入等抽象为一个动作。
type Action struct {
	Protocol string `json:"protocol,omitempty"` // 协议
	Request  string `json:"request,omitempty"`  // 请求
	Response string `json:"response,omitempty"` // 响应
}

// Session 一次上游调用称为一个会话。
type Session struct {
	Session string    `json:"session,omitempty"` // 会话 ID
	Inbound *Action   `json:"inbound,omitempty"` // 上游数据
	Actions []*Action `json:"actions,omitempty"` // 动作数据
}
