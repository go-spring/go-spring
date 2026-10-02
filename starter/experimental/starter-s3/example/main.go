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
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	starter "go-spring.org/starter-s3"
)

const bucket = "go-spring-example"

type Service struct {
	Client *starter.Client `autowire:"a"`
}

var manual = flag.Bool("manual", false, "run in manual verification mode (server stays up)")

func main() {
	flag.Parse()

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	if !*manual {
		go func() {
			time.Sleep(time.Millisecond * 500)
			runTest(svrBean.Interface().(*Service))
		}()
	} else {

		// Run the Go-Spring application.

		fmt.Println("=== Manual verification mode ===")
		fmt.Println("Server is running. Follow the README commands in another terminal.")
		fmt.Println("Press Ctrl+C to stop.")
	}
	gs.Run()
}

// runTest exercises the object-storage round trip: put an object, read it
// back, stat it, and remove it again.
func runTest(s *Service) {
	ctx := context.Background()

	exists, err := s.Client.BucketExists(ctx, bucket)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "BUCKETExists failed: %v", err)
		os.Exit(1)
	}
	if !exists {
		if err = s.Client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			log.Errorf(ctx, log.TagAppDef, "MakeBucket failed: %v", err)
			os.Exit(1)
		}
	}

	content := []byte("hello go-spring s3")
	if _, err = s.Client.PutObject(ctx, bucket, "hello.txt",
		bytes.NewReader(content), int64(len(content)),
		minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PutObject failed: %v", err)
		os.Exit(1)
	}

	obj, err := s.Client.GetObject(ctx, bucket, "hello.txt", minio.GetObjectOptions{})
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "GetObject failed: %v", err)
		os.Exit(1)
	}
	got, err := io.ReadAll(obj)
	_ = obj.Close()
	if err != nil || !bytes.Equal(got, content) {
		log.Errorf(ctx, log.TagAppDef, "READBACK failed: got=%q err=%v", got, err)
		os.Exit(1)
	}

	if _, err = s.Client.StatObject(ctx, bucket, "hello.txt", minio.StatObjectOptions{}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "StatObject failed: %v", err)
		os.Exit(1)
	}
	if err = s.Client.RemoveObject(ctx, bucket, "hello.txt", minio.RemoveObjectOptions{}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "RemoveObject failed: %v", err)
		os.Exit(1)
	}

	fmt.Println("Object round trip OK:", string(got))
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
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
