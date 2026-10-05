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

// payload.go builds the request body (and the signed URL where the channel
// demands it) for each supported webhook channel. Every builder returns a
// JSON body plus optional extra query parameters, so the send path stays one
// shape for all channels.

package StarterWebhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go-spring.org/stdlib/errutil"
)

// buildPayload renders the notification for the given channel: the JSON body,
// and extra URL query parameters (DingTalk's signed timestamp/sign pair).
func buildPayload(channel string, n *Notification, secret string, now time.Time) (body []byte, extraQuery url.Values, err error) {
	switch channel {
	case "", "generic":
		return genericBody(n), nil, nil
	case "dingtalk":
		q, serr := dingtalkSign(secret, now)
		if serr != nil {
			return nil, nil, serr
		}
		return dingtalkBody(n), q, nil
	case "feishu":
		body, err := feishuBody(n, secret, now)
		return body, nil, err
	case "wecom":
		return wecomBody(n), nil, nil
	case "slack":
		return slackBody(n), nil, nil
	default:
		return nil, nil, errutil.Explain(nil, "webhook: unknown channel %q (want generic|dingtalk|feishu|wecom|slack)", channel)
	}
}

// genericBody is a plain JSON POST: the lowest common denominator, useful for
// self-built receivers.
func genericBody(n *Notification) []byte {
	return marshal(map[string]any{"title": n.Title, "text": n.Text})
}

// dingtalkBody is the DingTalk group-robot markdown message.
func dingtalkBody(n *Notification) []byte {
	return marshal(map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"title": n.Title, "text": n.Title + "\n\n" + n.Text},
	})
}

// dingtalkSign appends the 加签 (HMAC) query pair DingTalk requires when the
// robot is configured with a secret.
func dingtalkSign(secret string, now time.Time) (url.Values, error) {
	if secret == "" {
		return nil, nil
	}
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "\n" + secret))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	return url.Values{"timestamp": {ts}, "sign": {sign}}, nil
}

// feishuBody is the Feishu/Lark custom-bot text message, with the signature
// folded into the body when a secret is configured.
func feishuBody(n *Notification, secret string, now time.Time) ([]byte, error) {
	m := map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": n.Title + "\n" + n.Text},
	}
	if secret != "" {
		ts := strconv.FormatInt(now.UnixMilli(), 10)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(ts + "\n" + secret))
		m["timestamp"] = ts
		m["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	return marshal(m), nil
}

// wecomBody is the WeCom (WeChat Work) group-robot markdown message.
func wecomBody(n *Notification) []byte {
	return marshal(map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"content": n.Title + "\n" + n.Text},
	})
}

// slackBody is the Slack incoming-webhook message.
func slackBody(n *Notification) []byte {
	return marshal(map[string]any{"text": n.Title + "\n" + n.Text})
}

// marshal never fails on map[string]any of strings; on the impossible failure
// it returns a fallback body naming the error.
func marshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf(`{"title":"marshal error","text":%q}`, err.Error()))
	}
	return b
}

// plainText renders the notification for logs.
func plainText(n *Notification) string {
	return strings.TrimSpace(n.Title + " " + n.Text)
}
