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

package golang

import (
	"github.com/go-spring/gs-http-gen/gen/generator"
	"github.com/go-spring/stdlib/errutil"
)

// Generator is the main generator.
type Generator struct{}

// Gen is the main entry point for generating code.
func (g *Generator) Gen(config *generator.Config) error {

	// Convert IDL to GoSpec
	spec, err := Convert(config.IDLSrcDir)
	if err != nil {
		return errutil.Explain(err, "convert IDL error")
	}

	// Generate type code
	for fileName := range spec.Files {
		if err = g.genType(config, fileName, spec); err != nil {
			return errutil.Explain(err, "generate type file %s error", fileName)
		}
	}

	// Generate server code if enabled in the configuration
	if config.EnableServer {
		if err = g.genServer(config, spec); err != nil {
			return errutil.Explain(err, "generate server file error")
		}
		if err = g.genValidate(config, spec); err != nil {
			return errutil.Explain(err, "generate validate file error")
		}
	}

	// Generate client code if enabled in the configuration
	if config.EnableClient {
		if err = g.genClient(config, spec); err != nil {
			return errutil.Explain(err, "generate client file error")
		}
	}

	// ...
	if err = g.genConfig(config, spec); err != nil {
		return errutil.Explain(err, "generate config file error")
	}

	return nil
}
