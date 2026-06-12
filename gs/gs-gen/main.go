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
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"go-spring.org/gs-gen/proto"
)

// Version of the gs-gen tool
const Version = "v0.0.2"

func main() {
	var version bool

	root := &cobra.Command{
		Use:          "gs-gen",
		Short:        "gen go server code from idl files",
		SilenceUsage: true,
	}

	root.Flags().BoolVar(&version, "version", false, "show version")

	root.RunE = func(cmd *cobra.Command, args []string) error {
		if version {
			fmt.Println(root.Short)
			fmt.Println(Version)
			return nil
		}

		if _, err := os.Stat("gs.json"); err != nil {
			log.Fatalln("gs.json not found")
		}

		entries, err := os.ReadDir("idl")
		if err != nil {
			log.Fatalln(err)
		}

		currDir, err := os.Getwd()
		if err != nil {
			log.Fatalln(err)
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			switch e.Name() {
			case "http":
				proto.GenHttp(currDir)
			default: // for linter
			}
		}

		return nil
	}

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
