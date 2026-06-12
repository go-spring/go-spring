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

package docgen

import (
	"go-spring.org/gs-http-gen/gen/docgen/openapi"
	"go-spring.org/stdlib/errutil"
)

// Config holds the configuration options for document generation.
type Config struct {
	IDLSrcDir string // Directory containing source IDL files
	OutputDir string // Directory where generated documents will be written
}

// GenOpenAPI generates an OpenAPI 3.0 document.
func GenOpenAPI(config *Config) error {
	return openapi.Gen((*openapi.Config)(config))
}

// GenSwagger generates a Swagger 2.0 document.
func GenSwagger(config *Config) error {
	return errutil.Explain(nil, "swagger 2.0 document generation is not supported yet")
}
