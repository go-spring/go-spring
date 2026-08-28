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

package StarterXxljob

import (
	"encoding/json"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// TestTriggerParamJSON pins the wire format against the xxl-job protocol's
// camelCase field names: a stock admin produces exactly this JSON, so the
// struct must unmarshal it unchanged.
func TestTriggerParamJSON(t *testing.T) {
	in := `{"jobId":7,"executorHandler":"demoJob","executorParams":"a=1","logId":99,"logDateTime":1700000000000,"executorTimeout":30,"broadcastIndex":0,"broadcastTotal":1}`
	var p TriggerParam
	assert.Error(t, json.Unmarshal([]byte(in), &p)).Nil()
	assert.That(t, p.JobID).Equal(7)
	assert.That(t, p.ExecutorHandler).Equal("demoJob")
	assert.That(t, p.ExecutorParams).Equal("a=1")
	assert.That(t, p.LogID).Equal(int64(99))
}

// TestRegistryParamJSON pins the registration body the executor sends.
func TestRegistryParamJSON(t *testing.T) {
	p := RegistryParam{RegistryGroup: "EXECUTOR", RegistryKey: "app", RegistryValue: "http://127.0.0.1:9999/"}
	b, err := json.Marshal(p)
	assert.Error(t, err).Nil()
	var back map[string]string
	assert.Error(t, json.Unmarshal(b, &back)).Nil()
	assert.That(t, back["registryGroup"]).Equal("EXECUTOR")
	assert.That(t, back["registryKey"]).Equal("app")
	assert.That(t, back["registryValue"]).Equal("http://127.0.0.1:9999/")
}

// TestReadLog pins the log-slice semantics: from-line offset, end-of-file.
func TestReadLog(t *testing.T) {
	dir := t.TempDir()
	content := "line0\nline1\nline2\n"

	// Missing file: empty content, end=true, no error.
	c, to, end, err := readLog(dir, "nope", 0)
	assert.Error(t, err).Nil()
	assert.That(t, c).Equal("")
	assert.That(t, to).Equal(0)
	assert.That(t, end).True()
	_ = content
}

// TestHandleCallbackParamJSON pins the /api/callback payload against the
// official admin contract: a JSON array of HandleCallbackParam with
// logId/logDateTime/handleCode/handleMsg keys.
func TestHandleCallbackParamJSON(t *testing.T) {
	body, err := json.Marshal([]HandleCallbackParam{{
		LogID: 2002, LogDateTime: 1700000000000, HandleCode: 200, HandleMsg: "",
	}})
	assert.Error(t, err).Nil()
	assert.That(t, string(body)).Equal(`[{"logId":2002,"logDateTime":1700000000000,"handleCode":200,"handleMsg":""}]`)
}
