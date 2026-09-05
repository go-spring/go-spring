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

// starter.go is the gs registration + glue concept of this starter: it
// registers the per-instance webhook notifier group under "${spring.webhook}"
// (thin, mail-style: stateless per call, no destroy hook) and owns the
// Notifier send path — payload build, resilience executor, trace span.
package StarterWebhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// Notification is one outbound webhook message. Title is the headline (shown
// bold/markdown by most receivers), Text is the body.
type Notification struct {
	Title string
	Text  string
}

// Notifier POSTs notifications to one webhook endpoint in one channel's
// payload format. It is safe to hold as a bean: each Send is a stateless
// HTTP request, so no long-lived connection is kept between calls (hence no
// destroy hook is needed).
type Notifier struct {
	cfg    Config
	client *http.Client
	exec   resilience.Executor
}

func init() {
	// Register multiple webhook notifiers as a group. Each instance is created
	// from the configuration under "${spring.webhook}", so adding a second
	// endpoint is a pure-config change. There is no default singleton —
	// select one by name (e.g. autowire:"alert").
	//
	// No destroy callback: each Send is a stateless HTTP request, so there is
	// nothing to release at shutdown.
	gs.Group("${spring.webhook}", newNotifier, nil)
}

// newNotifier builds a Notifier from config. There is deliberately no startup
// probe: the only universal probe would be a real POST, and sending a junk
// notification at boot is worse than failing on first use (see DESIGN).
func newNotifier(ctx *gs.ContextProvider, name string, c Config) (*Notifier, error) {
	if _, _, err := buildPayload(c.Channel, &Notification{}, c.Secret, time.Now()); err != nil {
		return nil, err
	}
	log.Debugf(ctx.Context, log.TagAppDef, "creating webhook notifier url=%s channel=%s", c.URL, c.Channel)

	exec := fault.WrapExecutor(resilience.ExecutorFor(resilience.ResourceLabel("webhook", name, c.Channel)))
	exec = resilience.WrapExecutor(exec, "webhook")
	return &Notifier{
		cfg:    c,
		client: &http.Client{Timeout: c.Timeout},
		exec:   exec,
	}, nil
}

// Channel reports the payload format this notifier speaks.
func (n *Notifier) Channel() string { return n.cfg.Channel }

// Send delivers the notification: it builds the channel payload, wraps the
// POST in a producer span, and routes it through the resilience executor
// (rate limit / circuit breaking / fault injection when starter-governance is
// imported; a transparent pass-through otherwise).
func (n *Notifier) Send(ctx context.Context, notification *Notification) error {
	body, extra, err := buildPayload(n.cfg.Channel, notification, n.cfg.Secret, time.Now())
	if err != nil {
		return err
	}

	endpoint, err := withQuery(n.cfg.URL, extra)
	if err != nil {
		return errutil.Explain(err, "webhook: invalid url %q", n.cfg.URL)
	}

	ctx, span := startSend(ctx, n.cfg.Channel, endpoint)
	post := func(ctx context.Context) error { return n.post(ctx, endpoint, body) }
	// A zero-value Notifier (built by hand in tests) has no executor; the
	// starter-built one always does, but keep Send usable either way.
	if n.exec != nil {
		err = n.exec.Execute(ctx, endpoint, post)
	} else {
		err = post(ctx)
	}
	EndSpan(span, err)
	return err
}

// post performs the HTTP POST and treats any non-2xx answer, or a vendor
// business error returned with HTTP 200 (DingTalk/WeCom "errcode", Feishu
// "code"/"StatusCode"), as an error. Channels without a business-code
// convention (generic, slack) judge success by the HTTP status alone.
func (n *Notifier) post(ctx context.Context, endpoint string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return errutil.Explain(err, "webhook: build request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return errutil.Explain(err, "webhook: post failed")
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errutil.Explain(nil, "webhook: %s returned %s: %s", n.cfg.Channel, resp.Status, string(respBody))
	}
	if msg := businessError(n.cfg.Channel, respBody); msg != "" {
		return errutil.Explain(nil, "webhook: %s returned %s: %s", n.cfg.Channel, resp.Status, msg)
	}
	return nil
}

// businessError extracts the vendor business error from a 200 response body,
// or returns "" when the body reports success (or the channel has no
// business-code convention). Unparseable bodies are treated as success: the
// HTTP status already said OK.
func businessError(channel string, body []byte) string {
	var v struct {
		Errcode    *int   `json:"errcode"`
		Errmsg     string `json:"errmsg"`
		Code       *int   `json:"code"`
		Msg        string `json:"msg"`
		StatusCode *int   `json:"StatusCode"`
		StatusMsg  string `json:"StatusMessage"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return ""
	}
	switch channel {
	case "dingtalk", "wecom": // {"errcode":0,"errmsg":"ok"}
		if v.Errcode != nil && *v.Errcode != 0 {
			return fmt.Sprintf("errcode=%d errmsg=%q", *v.Errcode, v.Errmsg)
		}
	case "feishu": // {"code":0,"msg":"success"} or legacy {"StatusCode":0,"StatusMessage":"success"}
		if v.Code != nil && *v.Code != 0 {
			return fmt.Sprintf("code=%d msg=%q", *v.Code, v.Msg)
		}
		if v.StatusCode != nil && *v.StatusCode != 0 {
			return fmt.Sprintf("StatusCode=%d StatusMessage=%q", *v.StatusCode, v.StatusMsg)
		}
	}
	return ""
}

// withQuery appends extra query parameters (DingTalk's signed pair) to a URL.
func withQuery(raw string, extra url.Values) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if len(extra) == 0 {
		return raw, nil
	}
	q := u.Query()
	for k, vs := range extra {
		for _, v := range vs {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
