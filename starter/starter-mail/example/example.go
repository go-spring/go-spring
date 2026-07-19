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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"

	StarterMail "go-spring.org/starter-mail"
)

// Service wires a single named mailer. Adding a second mailer would be a pure
// config change plus another autowire field — no code edit to the starter.
type Service struct {
	Notify *StarterMail.Mailer `autowire:"notify"`
}

func main() {
	// `s` is not referenced by any other bean, so register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	gs.Run()
}

func fail(format string, args ...any) {
	log.Errorf(context.Background(), log.TagAppDef, format, args...)
	os.Exit(1)
}

func runTest(s *Service) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send one HTML email with a plain-text alternative, an attachment, and
	// multiple recipients.
	err := s.Notify.Send(ctx, &StarterMail.Message{
		To:      []string{"alice@example.com", "bob@example.com"},
		Cc:      []string{"carol@example.com"},
		Subject: "Go-Spring starter-mail smoke test",
		Text:    "This is the plain-text fallback body.",
		HTML:    "<h1>Hello</h1><p>This is an <b>HTML</b> mail from starter-mail.</p>",
		Attachments: []StarterMail.Attachment{
			{Filename: "report.txt", Data: []byte("attachment payload: 42\n")},
		},
	})
	if err != nil {
		fail("send failed: %v", err)
	}

	// Verify delivery via MailHog's HTTP API: exactly one message should have
	// arrived, carrying the three recipients and an attachment.
	total, err := mailhogMessageCount()
	if err != nil {
		fail("querying MailHog failed: %v", err)
	}
	if total < 1 {
		fail("expected at least one delivered message, got %d", total)
	}

	fmt.Println("Response from server: mail sent and delivered OK")
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// mailhogMessageCount reads the MailHog inbox through its v2 API and returns the
// number of captured messages.
func mailhogMessageCount() (int, error) {
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8025/api/v2/messages", nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var body struct {
		Total int `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.Total, nil
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	err := os.Chdir(execDir)
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
