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

package StarterWebhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

// TestBuildPayloadChannels pins each channel's payload shape: the receiver
// must recognize the message the starter sends.
func TestBuildPayloadChannels(t *testing.T) {
	n := &Notification{Title: "cpu", Text: "90%"}

	// generic
	body, extra, err := buildPayload("generic", n, "", time.UnixMilli(0))
	assert.Error(t, err).Nil()
	assert.That(t, len(extra)).Equal(0)
	var g map[string]any
	assert.Error(t, json.Unmarshal(body, &g)).Nil()
	assert.That(t, g["title"]).Equal("cpu")
	assert.That(t, g["text"]).Equal("90%")

	// dingtalk: markdown body, unsigned without secret
	body, extra, err = buildPayload("dingtalk", n, "", time.UnixMilli(0))
	assert.Error(t, err).Nil()
	assert.That(t, len(extra)).Equal(0)
	var d map[string]any
	assert.Error(t, json.Unmarshal(body, &d)).Nil()
	assert.That(t, d["msgtype"]).Equal("markdown")

	// dingtalk signed: timestamp + sign query pair
	_, extra, err = buildPayload("dingtalk", n, "SECxxx", time.UnixMilli(1700000000000))
	assert.Error(t, err).Nil()
	assert.That(t, extra.Get("timestamp")).Equal("1700000000000")
	assert.That(t, extra.Get("sign") != "").True()

	// feishu signed: timestamp + sign folded into the body
	body, _, err = buildPayload("feishu", n, "s3cret", time.UnixMilli(1700000000000))
	assert.Error(t, err).Nil()
	var f map[string]any
	assert.Error(t, json.Unmarshal(body, &f)).Nil()
	assert.That(t, f["msg_type"]).Equal("text")
	assert.That(t, f["timestamp"]).Equal("1700000000000")
	assert.That(t, f["sign"] != "").True()

	// wecom + slack
	body, _, err = buildPayload("wecom", n, "", time.UnixMilli(0))
	assert.Error(t, err).Nil()
	assert.That(t, strings.Contains(string(body), `"msgtype":"markdown"`)).True()
	body, _, err = buildPayload("slack", n, "", time.UnixMilli(0))
	assert.Error(t, err).Nil()
	assert.That(t, strings.Contains(string(body), `"text":"cpu\n90%"`)).True()

	// unknown channel fails
	_, _, err = buildPayload("sms", n, "", time.UnixMilli(0))
	assert.That(t, err != nil).True()
}

// TestNotifierSendRoundTrip posts a generic notification to a local receiver
// and asserts the body that arrives on the wire.
func TestNotifierSendRoundTrip(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &Notifier{
		cfg:    Config{URL: srv.URL, Channel: "generic", Timeout: 2 * time.Second},
		client: &http.Client{Timeout: 2 * time.Second},
	}
	assert.Error(t, n.Send(context.Background(), &Notification{Title: "deploy", Text: "ok"})).Nil()
	assert.That(t, got["title"]).Equal("deploy")
	assert.That(t, got["text"]).Equal("ok")
}

// TestNotifierSendBadStatus proves a non-2xx receiver answer surfaces as an
// error instead of a silent success.
func TestNotifierSendBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	n := &Notifier{
		cfg:    Config{URL: srv.URL, Channel: "generic", Timeout: 2 * time.Second},
		client: &http.Client{Timeout: 2 * time.Second},
	}
	assert.That(t, n.Send(context.Background(), &Notification{Title: "x"}) != nil).True()
}

// TestNotifierSendBusinessError proves the vendor errcode convention (HTTP
// 200 + non-zero business code) surfaces as an error, and that a zero code
// plus channels without the convention still succeed.
func TestNotifierSendBusinessError(t *testing.T) {
	cases := []struct {
		name    string
		channel string
		body    string
		wantErr bool
	}{
		{"dingtalk ok", "dingtalk", `{"errcode":0,"errmsg":"ok"}`, false},
		{"dingtalk err", "dingtalk", `{"errcode":310000,"errmsg":"sign not match"}`, true},
		{"wecom err", "wecom", `{"errcode":93000,"errmsg":"invalid webhook url"}`, true},
		{"feishu new ok", "feishu", `{"code":0,"msg":"success"}`, false},
		{"feishu new err", "feishu", `{"code":19021,"msg":"sign match fail"}`, true},
		{"feishu legacy err", "feishu", `{"StatusCode":19024,"StatusMessage":"sign match fail"}`, true},
		{"slack ok text", "slack", "ok", false},
		{"generic passthrough", "generic", `whatever`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			n := &Notifier{
				cfg:    Config{URL: srv.URL, Channel: c.channel, Timeout: 2 * time.Second},
				client: &http.Client{Timeout: 2 * time.Second},
			}
			err := n.Send(context.Background(), &Notification{Title: "x"})
			if c.wantErr {
				assert.That(t, err != nil).True()
			} else {
				assert.Error(t, err).Nil()
			}
		})
	}
}

// TestWithQuery covers the DingTalk signed-URL append, including the
// preserve-existing-query case.
func TestWithQuery(t *testing.T) {
	got, err := withQuery("https://oapi.dingtalk.com/robot/send?access_token=t",
		map[string][]string{"timestamp": {"1700"}, "sign": {"X==/"}})
	assert.Error(t, err).Nil()
	assert.That(t, strings.Contains(got, "access_token=t")).True()
	assert.That(t, strings.Contains(got, "timestamp=1700")).True()
	assert.That(t, strings.Contains(got, "sign=")).True()

	got, err = withQuery("https://example.com/hook", nil)
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal("https://example.com/hook")
}
