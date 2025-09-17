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
	"embed"
	"fmt"
	"net/http"
	"time"

	"github.com/go-spring/spring-core/gs"
)

//go:embed chatAI.html
var files embed.FS

func main() {
	// Disable the write timeout for the HTTP server
	gs.Property("http.server.writeTimeout", "0")

	// Serve static files from the embedded file system under the "/public/" path
	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.FS(files))))

	// Handle the Server-Sent Events (SSE) endpoint
	http.HandleFunc("/chat/sse", func(w http.ResponseWriter, r *http.Request) {

		// Set the necessary HTTP headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		// Send an SSE message every second for 10 seconds
		for i := 0; i < 10; i++ {
			select {
			case <-r.Context().Done():
				// Exit the loop if the client disconnects
				return
			default:
				// Each SSE message must end with two newlines to be recognized correctly by the client
				// See more about SSE protocol: https://www.ruanyifeng.com/blog/2017/05/server-sent_events.html
				fmt.Fprintf(w, "data: Message %d at %s\n\n", i, time.Now().Format("15:04:05"))
				flusher.Flush()
				time.Sleep(1 * time.Second)
			}
		}
	})

	gs.Run()
}

// open http://127.0.0.1:9090/public/chatAI.html in the browser
