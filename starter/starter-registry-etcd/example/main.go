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

// Command example wires starter-registry-etcd into a Go-Spring application:
// blank-importing the starter plus a ${spring.registry.etcd.endpoints} entry
// registers this instance into etcd once the app is ready and deregisters it
// on shutdown.
//
// To make the mechanism visible without an external client, a goroutine waits
// for readiness, reads the registered keys back from etcd, prints what it
// finds, then SIGTERMs the app so the example self-terminates (exercising the
// deregister-on-shutdown path). It needs a reachable etcd at the configured
// address; check.sh starts one in Docker.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	// Blank-import registers the etcd registrar and the register-on-ready server.
	_ "go-spring.org/starter-registry-etcd"
)

// keyPrefix matches ${spring.registry.etcd.key-prefix} + service name.
const keyPrefix = "/services/orders/"

func main() {
	go func() {
		// Wait past readiness so registration has happened, then verify.
		time.Sleep(time.Second)
		verifyOnce()
		// One-shot: stop the app so the example terminates (and deregisters) on its own.
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()

	gs.Run()
}

// verifyOnce reads the registered keys from etcd and logs the instances found.
// It never exits non-zero on failure: the point is to show the call; check.sh
// asserts the end-to-end path separately.
func verifyOnce() {
	ctx := context.Background()
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "etcd client: %v", err)
		return
	}
	defer func() { _ = cli.Close() }()

	resp, err := cli.Get(ctx, keyPrefix, clientv3.WithPrefix())
	if err != nil {
		log.Warnf(ctx, log.TagAppDef, "query %q failed (is etcd reachable?): %v", keyPrefix, err)
		return
	}
	if len(resp.Kvs) == 0 {
		log.Warnf(ctx, log.TagAppDef, "prefix %q has no instances yet", keyPrefix)
		return
	}
	for _, kv := range resp.Kvs {
		fmt.Printf("registered key=%s value=%s\n", kv.Key, kv.Value)
	}
}

// init sets the working directory to this source file's directory so
// conf/app.properties loads regardless of where the process is launched.
func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	if err := os.Chdir(filepath.Dir(filename)); err != nil {
		panic(err)
	}
}
