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

// This example demonstrates the Nacos remote configuration provider and the
// version -> bean hot-reload link:
//
//  1. app.properties imports config from Nacos via
//     spring.app.imports=optional:nacos:.../gs-config-demo?...
//  2. A bean binds demo.message to a gs.Dync[string] field.
//  3. The example publishes a new value to Nacos; the provider's change
//     listener triggers a property refresh, and the bound field updates
//     without a restart.
//
// The publisher client below is built directly from the SDK rather than
// injected, keeping the demonstration focused on the provider and refresh
// link.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-nacos"
)

const (
	dataID    = "gs-config-demo"
	group     = "DEFAULT_GROUP"
	nacosAddr = "127.0.0.1:8848"
)

// Demo binds a dynamic configuration field sourced from the imported Nacos
// data id. It is registered as a root object so the container creates it
// eagerly.
type Demo struct {
	Message gs.Dync[string] `value:"${demo.message:=none}"`
}

func main() {
	demoBean := gs.Provide(&Demo{}).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(500 * time.Millisecond)
		runTest(demoBean.Interface().(*Demo))
	}()

	gs.Run()
}

func runTest(d *Demo) {
	ctx := context.Background()

	// Publish a new value for the imported data id via the Nacos HTTP open API.
	want := "hello-" + time.Now().Format("150405")
	if err := publish(want); err != nil {
		log.Errorf(ctx, log.TagAppDef, "publish config failed: %v", err)
		os.Exit(1)
	}

	// The provider's change listener triggers a property refresh, which
	// re-fetches the config and updates the bound gs.Dync field. Poll until
	// the new value is visible or time out.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if got := d.Message.Value(); got == want {
			fmt.Println("hot-reload observed:", got)
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Errorf(ctx, log.TagAppDef, "hot-reload timeout: message=%q want=%q", d.Message.Value(), want)
	os.Exit(1)
}

// publish writes demo.message=<value> to the Nacos data id via the HTTP open
// API (POST /nacos/v1/cs/configs).
func publish(value string) error {
	form := url.Values{
		"dataId":  {dataID},
		"group":   {group},
		"content": {"demo.message=" + value},
	}
	endpoint := "http://" + nacosAddr + "/nacos/v1/cs/configs"
	resp, err := http.PostForm(endpoint, form)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "true" {
		return fmt.Errorf("unexpected response: status=%d body=%q", resp.StatusCode, string(body))
	}
	return nil
}

// init sets the working directory to this source file's directory so relative
// config paths resolve correctly.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine source file path")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
}
