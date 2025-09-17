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

package bootstrap

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	// Register a function runner to initialize the remote configuration setup.
	gs.B.FuncRunner(initRemoteConfig).OnProfiles("online")
}

// initRemoteConfig initializes the remote configuration setup.
// It first attempts to retrieve remote config, then starts a background job
// to periodically refresh the configuration.
func initRemoteConfig() error {
	if err := getRemoteConfig(); err != nil {
		return err
	}
	// Register a function job to refresh the configuration.
	// A bean can be registered into the app during the bootstrap phase.
	gs.FuncJob(refreshRemoteConfig)
	return nil
}

// getRemoteConfig fetches and writes the remote configuration to a local file.
// It creates necessary directories and generates a properties file containing.
func getRemoteConfig() error {
	err := os.MkdirAll("./conf/remote", os.ModePerm)
	if err != nil {
		return err
	}

	const data = `
		server.addr=0.0.0.0:9090
		
		log.access.name=access.log
		log.access.dir=./log
		
		log.biz.name=biz.log
		log.biz.dir=./log
		
		log.dao.name=dao.log
		log.dao.dir=./log
		
		refresh_time=%v
	`

	const file = "conf/remote/app-online.properties"
	str := fmt.Sprintf(data, time.Now().UnixMilli())
	return os.WriteFile(file, []byte(str), os.ModePerm)
}

// refreshRemoteConfig runs a continuous loop to periodically update configuration.
// It refreshes every 500ms until context cancellation.
func refreshRemoteConfig(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done(): // Gracefully exit when context is canceled.
			// The context (ctx) is derived from the app instance.
			// When the app exits, the context is canceled,
			// allowing the loop to terminate gracefully.
			fmt.Println("config updater exit")
			return nil
		case <-time.After(time.Millisecond * 500):
			if err := getRemoteConfig(); err != nil {
				fmt.Println("get remote config error:", err)
				return err
			}
			// Refreshes the app configuration.
			if err := gs.RefreshProperties(); err != nil {
				fmt.Println("refresh properties error:", err)
				return err
			}
			fmt.Println("refresh properties success")
		}
	}
}
