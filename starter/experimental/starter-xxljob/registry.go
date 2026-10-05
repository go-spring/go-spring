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

// registry.go is the registration/heartbeat half of the protocol: the
// executor periodically registers itself with the admin and removes itself on
// shutdown. It also owns the small helpers the executor uses (log files,
// outbound IP).

package StarterXxljob

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// register starts the periodic registration/heartbeat loop and returns a stop
// function (which also removes the executor from the admin).
func (e *Executor) register(ctx context.Context) func() {
	tick := time.NewTicker(e.cfg.RegistryInterval)
	done := make(chan struct{})
	go func() {
		e.registryOnce(ctx)
		for {
			select {
			case <-tick.C:
				e.registryOnce(ctx)
			case <-done:
				tick.Stop()
				return
			}
		}
	}()
	return func() {
		close(done)
		e.registryRemove(context.Background())
	}
}

// registryOnce POSTs one registration/heartbeat to every admin address.
func (e *Executor) registryOnce(ctx context.Context) {
	body, _ := json.Marshal(RegistryParam{
		RegistryGroup: "EXECUTOR",
		RegistryKey:   e.cfg.AppName,
		RegistryValue: fmt.Sprintf("http://%s:%d/", e.ip, e.cfg.Port),
	})
	for _, base := range e.cfg.AdminAddresses {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/registry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if e.cfg.AccessToken != "" {
			req.Header.Set("XXL-JOB-ACCESS-TOKEN", e.cfg.AccessToken)
		}
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}
}

// registryRemove de-registers the executor from every admin.
func (e *Executor) registryRemove(ctx context.Context) {
	body, _ := json.Marshal(RegistryParam{
		RegistryGroup: "EXECUTOR",
		RegistryKey:   e.cfg.AppName,
		RegistryValue: fmt.Sprintf("http://%s:%d/", e.ip, e.cfg.Port),
	})
	for _, base := range e.cfg.AdminAddresses {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/registry/remove", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if e.cfg.AccessToken != "" {
			req.Header.Set("XXL-JOB-ACCESS-TOKEN", e.cfg.AccessToken)
		}
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}
}

// outboundIP returns this host's preferred outbound address, for the
// registration value the admin must be able to dial back to.
func outboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String(), nil
}

// ensureLogDir creates the task-log directory if absent.
func ensureLogDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// readLog returns the task log slice the admin asked for, or an error.
func readLog(dir, logID string, fromLine int) (content string, toLine int, end bool, err error) {
	b, err := os.ReadFile(filepath.Join(dir, logID+".log"))
	if os.IsNotExist(err) {
		return "", 0, true, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if fromLine >= len(lines) {
		return "", len(lines), true, nil
	}
	toLine = len(lines)
	return strings.Join(lines[fromLine:], "\n"), toLine, true, nil
}

// unused: strconv/time kept for the doc example of the protocol timestamps.
var _ = strconv.Itoa
var _ = time.Now
