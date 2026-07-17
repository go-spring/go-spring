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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/gomodule/redigo/redis"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterRedigo "go-spring.org/starter-redigo"
)

func init() {
	StarterRedigo.RegisterDriver("AnotherRedisDriver", &AnotherRedisDriver{})
}

// AnotherRedisDriver is a custom implementation of the Driver interface.
type AnotherRedisDriver struct{}

func (AnotherRedisDriver) CreateClient(c StarterRedigo.Config) (*redis.Pool, error) {
	log.Infof(context.Background(), log.TagAppDef, "AnotherRedisDriver::CreateClient")
	return &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", c.Addr, redis.DialPassword(c.Password))
		},
	}, nil
}

type Service struct {
	Redis *redis.Pool `autowire:"cache"`
}

func main() {
	// You can change the `driver` property in the configuration file
	// and check the used Redis driver via logs.

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	// Define a handler to GET a Redis key value.
	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		c := s.Redis.Get()
		defer func() { _ = c.Close() }()
		str, err := redis.String(c.Do("GET", "key"))
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(str))
	})

	// Define a handler to SET a Redis key value.
	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		c := s.Redis.Get()
		defer func() { _ = c.Close() }()
		str, err := redis.String(c.Do("SET", "key", "value"))
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(str))
	})

	// Define a handler to INCR a Redis counter key.
	http.HandleFunc("/incr", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		c := s.Redis.Get()
		defer func() { _ = c.Close() }()
		n, err := redis.Int(c.Do("INCR", "counter"))
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(strconv.Itoa(n)))
	})

	// Define a handler to inspect a key's TTL.
	http.HandleFunc("/ttl", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		c := s.Redis.Get()
		defer func() { _ = c.Close() }()
		ttl, err := redis.Int(c.Do("TTL", "key"))
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(strconv.Itoa(ttl)))
	})

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage:
	//
	// ~ curl http://127.0.0.1:9090/get
	// redigo: nil returned%
	// ~ curl http://127.0.0.1:9090/set
	// OK%
	// ~ curl http://127.0.0.1:9090/get
	// value%
}

func runTest(s *Service) {
	ctx := context.Background()
	c := s.Redis.Get()
	defer func() { _ = c.Close() }()

	// Feature 1: String SET/GET.
	if _, err := redis.String(c.Do("SET", "key", "value")); err != nil {
		log.Errorf(ctx, log.TagAppDef, "SET failed: %v", err)
		os.Exit(1)
	}
	v, err := redis.String(c.Do("GET", "key"))
	if err != nil || v != "value" {
		log.Errorf(ctx, log.TagAppDef, "GET failed: v=%q err=%v", v, err)
		os.Exit(1)
	}

	// Feature 2: INCR counter — reset then increment three times.
	if _, err := c.Do("DEL", "counter"); err != nil {
		log.Errorf(ctx, log.TagAppDef, "DEL counter failed: %v", err)
		os.Exit(1)
	}
	var n int
	for i := 0; i < 3; i++ {
		n, err = redis.Int(c.Do("INCR", "counter"))
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "INCR failed: %v", err)
			os.Exit(1)
		}
	}
	if n != 3 {
		log.Errorf(ctx, log.TagAppDef, "INCR final value expected 3, got %d", n)
		os.Exit(1)
	}

	// Feature 3: EXPIRE + TTL.
	if _, err := redis.String(c.Do("SET", "ttl-key", "ttl-value")); err != nil {
		log.Errorf(ctx, log.TagAppDef, "SET ttl-key failed: %v", err)
		os.Exit(1)
	}
	if _, err := c.Do("EXPIRE", "ttl-key", 30); err != nil {
		log.Errorf(ctx, log.TagAppDef, "EXPIRE failed: %v", err)
		os.Exit(1)
	}
	ttl, err := redis.Int(c.Do("TTL", "ttl-key"))
	if err != nil || ttl <= 0 || ttl > 30 {
		log.Errorf(ctx, log.TagAppDef, "TTL out of range: ttl=%v err=%v", ttl, err)
		os.Exit(1)
	}

	fmt.Println("Response from server:", v, "counter:", n, "ttl:", ttl)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory
// where this source file resides.
// This ensures that any relative file operations are based on the source file location,
// not the process launch path.
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
