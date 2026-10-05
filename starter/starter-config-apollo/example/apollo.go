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

// This file is the example's in-process mock Apollo config service. It stands in
// for a real Apollo stack (which would need MySQL plus configservice/admin/
// portal) by serving exactly the endpoints agollo drives, so the example can
// exercise the cold load and the notification-driven hot reload with no docker.

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

const mockAddr = "127.0.0.1:18080"

// apolloStore is the mock service's mutable state: the namespace's message and
// the notification id agollo long-polls on. Publishing bumps the id so a
// waiter's poll returns, mirroring Apollo's notifications/v2 contract.
type apolloStore struct {
	mu       sync.Mutex
	message  string
	notifyID int64
}

func (s *apolloStore) set(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
	s.notifyID++
}

func (s *apolloStore) get() (string, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.message, s.notifyID
}

var store = &apolloStore{message: "hello-from-apollo"}

// mockConfigService serves the endpoints agollo drives:
//
//	/services/config (meta service discovery),
//	/configfiles/json/{appId}/{cluster}/{ns} (cold load),
//	/configs/{appId}/{cluster}/{ns} (the re-fetch after a notification),
//	/notifications/v2 (the long poll).
//
// POST /publish?value=... is a knob for manual runs. It returns only after the
// listener is bound, so the caller can start the app knowing the mock accepts.
func mockConfigService() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stderr, "[mock-apollo] %s %s\n", r.Method, r.URL.String())
		switch r.URL.Path {
		case "/services/config":
			fmt.Fprintf(w, `[{"appName":"demo","instanceId":"mock","homepageUrl":"http://%s"}]`, mockAddr)
		case "/configfiles/json/demo/default/application":
			// The /configfiles/json/... endpoint returns the raw JSON object
			// (unmarshalled straight into the configurations map), not the
			// ApolloConfig envelope.
			msg, _ := store.get()
			fmt.Fprintf(w, `{"demo.message":%q}`, msg)
		case "/configs/demo/default/application":
			// The re-fetch that follows a notification returns the ApolloConfig
			// envelope, whose "configurations" map is what agollo caches.
			msg, id := store.get()
			fmt.Fprintf(w, `{"appId":"demo","cluster":"default","namespaceName":"application",`+
				`"configurations":{"demo.message":%q},"releaseKey":"%d"}`, msg, id)
		case "/notifications/v2":
			notifyLongPoll(w, r)
		case "/publish":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			store.set(r.URL.Query().Get("value"))
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	ln, err := net.Listen("tcp", mockAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[mock-apollo] listen on %s failed: %v\n", mockAddr, err)
		os.Exit(1)
	}
	go func() {
		if err := http.Serve(ln, mux); err != nil {
			fmt.Fprintf(os.Stderr, "[mock-apollo] serve failed: %v\n", err)
			os.Exit(1)
		}
	}()
}

// notifyLongPoll implements Apollo's notifications/v2 contract: it holds the
// request until the namespace's notification id advances past the client's, or
// ~30s elapse (then 304, nothing changed). Returning immediately would just
// make agollo re-poll every 2s; holding mirrors the real server.
func notifyLongPoll(w http.ResponseWriter, r *http.Request) {
	var notifies []struct {
		NamespaceName  string `json:"namespaceName"`
		NotificationID int64  `json:"notificationId"`
	}
	notifications := r.URL.Query().Get("notifications")
	if err := json.Unmarshal([]byte(notifications), &notifies); err != nil || len(notifies) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ns, clientID := notifies[0].NamespaceName, notifies[0].NotificationID

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, id := store.get(); id > clientID {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[{"namespaceName":%q,"notificationId":%d}]`, ns, id)
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	w.WriteHeader(http.StatusNotModified)
}
