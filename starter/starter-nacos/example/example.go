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
	"syscall"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/log"
	"go-spring.org/spring/gs"

	_ "go-spring.org/starter-nacos"
)

const (
	dataID       = "key"
	dataIDListen = "key-listen"
	group        = "DEFAULT_GROUP"
	serviceName  = "echo"
	instanceIP   = "127.0.0.1"
	instancePort = 9090
)

type Service struct {
	Config config_client.IConfigClient `autowire:"__default__"`
	Naming naming_client.INamingClient `autowire:"__default__"`
}

func main() {

	// Here `s` is not referenced by any other object,
	// so we need to register it as a root object.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		content, err := s.Config.GetConfig(vo.ConfigParam{DataId: dataID, Group: group})
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte(content))
	})

	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		_, err := s.Config.PublishConfig(vo.ConfigParam{DataId: dataID, Group: group, Content: "value"})
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte("OK"))
	})

	// Define a handler to look up service instances via the naming client.
	http.HandleFunc("/discover", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		svc, err := s.Naming.GetService(vo.GetServiceParam{ServiceName: serviceName, GroupName: group})
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = fmt.Fprintf(w, "hosts=%d", len(svc.Hosts))
	})

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svrBean.Interface().(*Service))
	}()

	// Run the Go-Spring application.
	gs.Run()

	// Example usage:
	//
	// ~ curl http://127.0.0.1:9090/set
	// OK%
	// ~ curl http://127.0.0.1:9090/get
	// value%
	// ~ curl http://127.0.0.1:9090/discover
	// hosts=1%
}

func runTest(s *Service) {
	ctx := context.Background()

	// Feature 1: config publish + get.
	// Nacos publishes asynchronously, so poll GetConfig until the value is visible.
	if _, err := s.Config.PublishConfig(vo.ConfigParam{DataId: dataID, Group: group, Content: "value"}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PublishConfig failed: %v", err)
		os.Exit(1)
	}
	content, err := waitForConfig(s.Config, dataID, group, "value", 3*time.Second)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "GetConfig failed: content=%q err=%v", content, err)
		os.Exit(1)
	}

	// Feature 2: config listen — observe async change notification.
	changeCh := make(chan string, 1)
	if err := s.Config.ListenConfig(vo.ConfigParam{
		DataId: dataIDListen,
		Group:  group,
		OnChange: func(namespace, group, dataId, data string) {
			select {
			case changeCh <- data:
			default:
			}
		},
	}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "ListenConfig failed: %v", err)
		os.Exit(1)
	}
	// Give the listener a brief moment to register before publishing.
	time.Sleep(500 * time.Millisecond)
	if _, err := s.Config.PublishConfig(vo.ConfigParam{DataId: dataIDListen, Group: group, Content: "changed"}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "PublishConfig (listen) failed: %v", err)
		os.Exit(1)
	}
	var received string
	select {
	case received = <-changeCh:
	case <-time.After(5 * time.Second):
		log.Errorf(ctx, log.TagAppDef, "ListenConfig callback timeout after 5s")
		os.Exit(1)
	}
	if received != "changed" {
		log.Errorf(ctx, log.TagAppDef, "ListenConfig payload mismatch: got=%q want=%q", received, "changed")
		os.Exit(1)
	}

	// Feature 3: service register + discovery via the naming client.
	ok, err := s.Naming.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          instanceIP,
		Port:        instancePort,
		ServiceName: serviceName,
		GroupName:   group,
		Weight:      1,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
	})
	if err != nil || !ok {
		log.Errorf(ctx, log.TagAppDef, "RegisterInstance failed: ok=%v err=%v", ok, err)
		os.Exit(1)
	}
	hosts, err := waitForInstance(s.Naming, serviceName, group, instanceIP, instancePort, 5*time.Second)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "GetService discovery failed: err=%v", err)
		os.Exit(1)
	}

	fmt.Println("Response from server:", content, "changed:", received, "hosts:", hosts)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// waitForConfig polls GetConfig until it returns the expected content or the timeout elapses.
func waitForConfig(cli config_client.IConfigClient, dataID, group, want string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var last string
	var lastErr error
	for time.Now().Before(deadline) {
		content, err := cli.GetConfig(vo.ConfigParam{DataId: dataID, Group: group})
		last, lastErr = content, err
		if err == nil && content == want {
			return content, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	if lastErr != nil {
		return last, lastErr
	}
	return last, fmt.Errorf("expected %q, got %q", want, last)
}

// waitForInstance polls GetService until an instance matching ip/port is found or the timeout elapses.
func waitForInstance(cli naming_client.INamingClient, serviceName, group, ip string, port uint64, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	var lastCount int
	var lastErr error
	for time.Now().Before(deadline) {
		svc, err := cli.GetService(vo.GetServiceParam{ServiceName: serviceName, GroupName: group})
		lastErr = err
		if err == nil {
			lastCount = len(svc.Hosts)
			for _, h := range svc.Hosts {
				if h.Ip == ip && h.Port == port {
					return lastCount, nil
				}
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	if lastErr != nil {
		return lastCount, lastErr
	}
	return lastCount, fmt.Errorf("instance %s:%d not found in service %q", ip, port, serviceName)
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
