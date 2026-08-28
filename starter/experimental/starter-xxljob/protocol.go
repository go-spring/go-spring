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

// protocol.go is the xxl-job executor<->admin wire format: the JSON request /
// response bodies the executor's callback server receives and the admin REST
// endpoints it calls. Field names match the xxl-job protocol (camelCase), so
// the structs round-trip against a stock xxl-job-admin unchanged.
package StarterXxljob

import "time"

// TriggerParam is the /run request body the admin POSTs to the executor when a
// job fires.
type TriggerParam struct {
	JobID                 int    `json:"jobId"`
	ExecutorHandler       string `json:"executorHandler"`
	ExecutorParams        string `json:"executorParams"`
	ExecutorBlockStrategy string `json:"executorBlockStrategy"`
	ExecutorTimeout       int    `json:"executorTimeout"` // seconds
	LogID                 int64  `json:"logId"`
	LogDateTime           int64  `json:"logDateTime"` // ms
	GlueType              string `json:"glueType"`
	GlueSource            string `json:"glueSource"`
	GlueUpdatetime        int64  `json:"glueUpdatetime"`
	BroadcastIndex        int    `json:"broadcastIndex"`
	BroadcastTotal        int    `json:"broadcastTotal"`
}

// TriggerResponse is the /run response body.
type TriggerResponse struct {
	Code    int    `json:"code"` // 200 = success, 500 = failure
	Msg     string `json:"msg"`
	Content *struct {
		LogID int64 `json:"logId"`
	} `json:"content"`
}

// KillParam is the /kill request body the admin POSTs to the executor.
type KillParam struct {
	JobID int64 `json:"jobId"`
}

// IdleBeatParam is the /idleBeat request body the admin POSTs to the executor.
type IdleBeatParam struct {
	JobID int64 `json:"jobId"`
}

// HandleCallbackParam is one entry of the JSON array the executor POSTs to the
// admin's /api/callback when a task completes. This is the official shape
// (xxl-job admin's com.xx.job.core.biz.model.HandleCallbackParam); the admin
// rejects anything else silently.
type HandleCallbackParam struct {
	LogID       int64  `json:"logId"`
	LogDateTime int64  `json:"logDateTime"`
	HandleCode  int    `json:"handleCode"` // 200 = success, 500 = failure
	HandleMsg   string `json:"handleMsg"`
}

// RegistryParam is the executor's own registration/heartbeat body sent to the
// admin's /api/registry endpoint.
type RegistryParam struct {
	RegistryGroup string `json:"registryGroup"` // EXECUTOR
	RegistryKey   string `json:"registryKey"`   // app name
	RegistryValue string `json:"registryValue"` // "http://ip:port/"
}

// LogResult is the /log response body the executor sends back to the admin
// when it asks for a task's rolling log file (NOT the /api/callback shape —
// see HandleCallbackParam).
type LogResult struct {
	FromLineNum int    `json:"fromLineNum"`
	ToLineNum   int    `json:"toLineNum"`
	LogContent  string `json:"logContent"`
	IsEnd       bool   `json:"isEnd"` // true on the final callback
}

// handlerResult is the executor-internal outcome of one task run.
type handlerResult struct {
	code int    // 200 / 500
	msg  string // error detail, empty on success
}

// now is a tiny indirection over time.Now so tests can pin the clock if
// needed without touching the protocol structs.
var now = time.Now
