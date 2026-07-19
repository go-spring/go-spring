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

// Command inventory is service B of the full-stack reference app: the downstream
// the order service (A) calls during its Saga. It is a plain gin HTTP service
// that reserves and releases stock, registers itself into Consul as "inventory"
// so A can discover it, and continues any trace A propagates so a single request
// is one trace across both hops.
//
//	POST /reserve  -> { "token": "<id>" }   reserves one unit, returns a token
//	POST /release  -> body: token           releases a previously reserved unit
//
// The /release endpoint is what makes Saga compensation observable end to end:
// when A's Saga fails after reserving, its compensation calls /release here.
package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	StarterGin "go-spring.org/starter-gin"
	StarterOTel "go-spring.org/starter-otel"

	// Actuator management server (probes) + otel (tracing/metrics + log link).
	_ "go-spring.org/starter-actuator"
	// Register this instance into Consul on ready, deregister on shutdown.
	_ "go-spring.org/starter-registry-consul"
)

// bizTag routes this service's log lines under a business tag so they are easy
// to correlate with a request's trace_id in the smoke output.
var bizTag = log.RegisterBizTag("inventory", "")

func init() {
	gs.Provide(&store{})

	// The starter owns the *gin.Engine and its HTTP server (${spring.gin.server});
	// we only wire routes and the trace-continuation middleware onto it.
	gs.Provide(func(s *store) StarterGin.RouterRegister {
		return func(e *gin.Engine) {
			e.Use(traceMiddleware)
			e.POST("/reserve", s.reserve)
			e.POST("/release", s.release)
		}
	})
}

// store is the in-memory stock ledger. It is intentionally trivial: the point of
// the reference app is the cross-service orchestration, not a real inventory.
type store struct {
	mu       sync.Mutex
	nextID   int
	reserved map[string]bool
}

func (s *store) init() {
	if s.reserved == nil {
		s.reserved = make(map[string]bool)
	}
}

// reserve books one unit and returns a reservation token the caller must present
// to /release. It is the forward action of the order Saga's first step.
func (s *store) reserve(c *gin.Context) {
	s.mu.Lock()
	s.init()
	s.nextID++
	token := "res-" + strconv.Itoa(s.nextID)
	s.reserved[token] = true
	held := len(s.reserved)
	s.mu.Unlock()

	log.Infof(c.Request.Context(), bizTag, "reserved token=%s held=%d", token, held)
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// release frees a previously reserved unit. It is the compensation of the order
// Saga's reserve step, so it MUST be idempotent: releasing an unknown or
// already-released token is a success, not an error.
func (s *store) release(c *gin.Context) {
	token := c.PostForm("token")
	if token == "" {
		token = c.Query("token")
	}

	s.mu.Lock()
	s.init()
	_, existed := s.reserved[token]
	delete(s.reserved, token)
	held := len(s.reserved)
	s.mu.Unlock()

	log.Infof(c.Request.Context(), bizTag, "released token=%s existed=%t held=%d", token, existed, held)
	c.JSON(http.StatusOK, gin.H{"released": token})
}

// traceMiddleware extracts the W3C trace context the caller injected (starter-otel
// installs the global propagator) and starts a server span, so this service's
// spans and logs join the caller's trace instead of starting a disconnected one.
func traceMiddleware(c *gin.Context) {
	ctx, span := StarterOTel.StartServerSpan(c.Request.Context(), c.Request.Header, "inventory", c.Request.Method+" "+c.FullPath())
	defer span.End()
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func main() {
	gs.Run()
}

// init sets the working directory to this source file's directory so the
// relative conf/app.properties path resolves regardless of where the process is
// launched from.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller")
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
	wd, _ := os.Getwd()
	fmt.Println("inventory workdir:", wd)
}
