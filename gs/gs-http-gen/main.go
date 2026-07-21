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
	"os"

	"github.com/spf13/cobra"
	"go-spring.org/gs-http-gen/gen"
	"go-spring.org/gs-http-gen/gen/docgen"
	"go-spring.org/gs-http-gen/gen/generator"
	"go-spring.org/gs-http-gen/lib/version"
)

func main() {
	var (
		showVersion   bool
		language      string
		outputDir     string
		goPackage     string
		enableServer  bool
		enableClient  bool
		enableSwagger bool
		enableOpenAPI bool
	)

	root := &cobra.Command{
		Use:   "gs-http-gen",
		Short: "A code generation tool for HTTP services based on IDL files",
		Long: `gs-http-gen is a code generation tool that reads service definitions
from IDL files and generates server and/or client code in Go (default),
PHP, Java, or other supported languages.`,
		SilenceUsage: true,
	}

	root.Flags().BoolVar(&showVersion, "version", false, "Display the version of gs-http-gen tool")
	root.Flags().StringVar(&language, "language", "go", "Target language for code generation (go, php, java, etc.)")
	root.Flags().BoolVar(&enableServer, "server", false, "Generate server-side code")
	root.Flags().BoolVar(&enableClient, "client", false, "Generate client-side code")
	root.Flags().BoolVar(&enableSwagger, "swagger", false, "Generate Swagger 2.0 document")
	root.Flags().BoolVar(&enableOpenAPI, "openapi", false, "Generate OpenAPI 3.0 document")
	root.Flags().StringVar(&outputDir, "output", ".", "Output directory for generated code (default: current directory)")
	root.Flags().StringVar(&goPackage, "go_package", "proto", "Go package name for generated code")

	root.RunE = func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(root.Short)
			fmt.Println(version.ToolVersion)
			return nil
		}

		if enableSwagger && enableOpenAPI {
			return fmt.Errorf("--swagger and --openapi cannot be used together")
		}
		if (enableSwagger || enableOpenAPI) && (enableServer || enableClient) {
			return fmt.Errorf("--swagger/--openapi cannot be used with --server or --client")
		}
		if enableSwagger {
			return docgen.GenSwagger(&docgen.Config{
				IDLSrcDir: ".",
				OutputDir: outputDir,
			})
		}
		if enableOpenAPI {
			return docgen.GenOpenAPI(&docgen.Config{
				IDLSrcDir: ".",
				OutputDir: outputDir,
			})
		}

		config := &generator.Config{
			IDLSrcDir:    ".",
			OutputDir:    outputDir,
			EnableServer: enableServer,
			EnableClient: enableClient,
			GoPackage:    goPackage,
			ToolVersion:  version.ToolVersion,
		}
		return gen.Gen(language, config)
	}

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
