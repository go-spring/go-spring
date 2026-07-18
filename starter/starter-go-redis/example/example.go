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

	"github.com/redis/go-redis/v9"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterGoRedis "go-spring.org/starter-go-redis"
)

func init() {
	StarterGoRedis.RegisterDriver("AnotherRedisDriver", &AnotherRedisDriver{})
}

// AnotherRedisDriver is a custom implementation of the Driver interface.
type AnotherRedisDriver struct{}

func (AnotherRedisDriver) CreateClient(c StarterGoRedis.Config) (*redis.Client, error) {
	log.Infof(context.Background(), log.TagAppDef, "AnotherRedisDriver::CreateClient")
	return redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
	}), nil
}

type Service struct {
	Redis          *redis.Client        `autowire:"cache"`
	DiscoveryRedis *redis.Client        `autowire:"discovery"`
	SentinelRedis  *redis.Client        `autowire:"sentinel"`
	ClusterRedis   *redis.ClusterClient `autowire:"cluster"`
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
		str, err := s.Redis.Get(r.Context(), "key").Result()
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(str))
	})

	// Define a handler to SET a Redis key value.
	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		str, err := s.Redis.Set(r.Context(), "key", "value", 0).Result()
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(str))
	})

	// Define a handler to INCR a Redis counter key.
	http.HandleFunc("/incr", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		n, err := s.Redis.Incr(r.Context(), "counter").Result()
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(strconv.FormatInt(n, 10)))
	})

	// Define a handler to inspect a key's TTL.
	http.HandleFunc("/ttl", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		d, err := s.Redis.TTL(r.Context(), "key").Result()
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(d.String()))
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
	// redis: nil%
	// ~ curl http://127.0.0.1:9090/set
	// OK%
	// ~ curl http://127.0.0.1:9090/get
	// value%
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 1: String SET/GET.
	if _, err := s.Redis.Set(ctx, "key", "value", 0).Result(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "SET failed: %v", err)
		os.Exit(1)
	}
	v, err := s.Redis.Get(ctx, "key").Result()
	if err != nil || v != "value" {
		log.Errorf(ctx, log.TagAppDef, "GET failed: v=%q err=%v", v, err)
		os.Exit(1)
	}

	// Feature 2: INCR counter — reset then increment three times.
	if _, err := s.Redis.Del(ctx, "counter").Result(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "DEL counter failed: %v", err)
		os.Exit(1)
	}
	var n int64
	for i := 0; i < 3; i++ {
		n, err = s.Redis.Incr(ctx, "counter").Result()
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
	if _, err := s.Redis.Set(ctx, "ttl-key", "ttl-value", 0).Result(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "SET ttl-key failed: %v", err)
		os.Exit(1)
	}
	if _, err := s.Redis.Expire(ctx, "ttl-key", 30*time.Second).Result(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "EXPIRE failed: %v", err)
		os.Exit(1)
	}
	ttl, err := s.Redis.TTL(ctx, "ttl-key").Result()
	if err != nil || ttl <= 0 || ttl > 30*time.Second {
		log.Errorf(ctx, log.TagAppDef, "TTL out of range: ttl=%v err=%v", ttl, err)
		os.Exit(1)
	}

	fmt.Println("Response from server:", v, "counter:", n, "ttl:", ttl)

	// Feature 4: the discovery-backed client. Its address came from the
	// registered discovery backend (service-name=redis-cluster), not from
	// conf's dummy addr, so a successful round-trip proves discovery is wired.
	if _, err := s.DiscoveryRedis.Set(ctx, "disc-key", "disc-value", 0).Result(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "discovery SET failed: %v", err)
		os.Exit(1)
	}
	dv, err := s.DiscoveryRedis.Get(ctx, "disc-key").Result()
	if err != nil || dv != "disc-value" {
		log.Errorf(ctx, log.TagAppDef, "discovery GET failed: v=%q err=%v", dv, err)
		os.Exit(1)
	}
	fmt.Println("Response from discovered server:", dv)

	// Feature 5: health check + pool monitoring. The go-redis client exposes
	// Ping for readiness probes and PoolStats() for runtime connection-pool
	// monitoring — no starter wrapper needed, they are read straight off the
	// autowired client.
	if err := s.Redis.Ping(ctx).Err(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "health ping failed: %v", err)
		os.Exit(1)
	}
	stats := s.Redis.PoolStats()
	fmt.Println("Pool stats:", "hits:", stats.Hits, "misses:", stats.Misses,
		"total-conns:", stats.TotalConns, "idle-conns:", stats.IdleConns)

	// Feature 6: sentinel topology. The `sentinel` instance (mode=sentinel in
	// conf) connects to the master resolved via the sentinels, but is still a
	// plain *redis.Client, so the command surface is identical to single mode.
	if _, err := s.SentinelRedis.Set(ctx, "sentinel-key", "sentinel-value", 0).Result(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "sentinel SET failed: %v", err)
		os.Exit(1)
	}
	sv, err := s.SentinelRedis.Get(ctx, "sentinel-key").Result()
	if err != nil || sv != "sentinel-value" {
		log.Errorf(ctx, log.TagAppDef, "sentinel GET failed: v=%q err=%v", sv, err)
		os.Exit(1)
	}
	fmt.Println("Response from sentinel master:", sv)

	// Feature 7: cluster topology. The `cluster` instance (mode=cluster in conf)
	// is a *redis.ClusterClient — a distinct bean type — that routes keys across
	// the cluster's hash slots. redisotel attaches per node via OnNewNode.
	if _, err := s.ClusterRedis.Set(ctx, "cluster-key", "cluster-value", 0).Result(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "cluster SET failed: %v", err)
		os.Exit(1)
	}
	cv, err := s.ClusterRedis.Get(ctx, "cluster-key").Result()
	if err != nil || cv != "cluster-value" {
		log.Errorf(ctx, log.TagAppDef, "cluster GET failed: v=%q err=%v", cv, err)
		os.Exit(1)
	}
	fmt.Println("Response from cluster:", cv)

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
