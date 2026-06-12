/*
 * Copyright 2024 The Go-Spring Authors.
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

// Package gs_conf provides a layered configuration system for Go-Spring
// applications. It merges multiple configuration sources into a single
// layered property set, supporting profile-specific files and optional
// imports of additional configuration files.
//
// This implementation follows the Spring Boot layered configuration model,
// where configuration sources have a well-defined precedence order, and
// higher-priority sources override lower-priority ones when the same key
// appears in multiple places.
//
// # Precedence Order (Highest → Lowest)
//
// Configuration sources are organized into layers according to priority:
//
//  1. **Command-line arguments** - Highest precedence
//  2. **Operating system environment variables**
//  3. **Profile-specific configuration** (`app-{profile}.yaml` etc.)
//     - Imports declared in profile configuration files (`spring.app.imports`)
//     - The profile-specific configuration file itself
//  4. **Application base configuration** (`app.yaml` etc.)
//     - Imports declared in application configuration files (`spring.app.imports`)
//     - The base application configuration file itself
//  5. **Built-in default properties** - Lowest precedence
package gs_conf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/stdlib/flatten"
)

// AppConfig represents the layered configuration of an application.
type AppConfig struct {
	Properties *flatten.Properties
}

// NewAppConfig creates a new AppConfig instance.
func NewAppConfig() *AppConfig {
	return &AppConfig{
		Properties: flatten.NewProperties(nil),
	}
}

// Refresh refreshes the configuration by merging multiple sources.
func (c *AppConfig) Refresh() (flatten.Storage, error) {
	cmd, err := extractCmdArgs()
	if err != nil {
		return nil, err
	}

	env, err := extractEnvironments()
	if err != nil {
		return nil, err
	}

	l := &flatten.LayeredStorage{}
	l.AddStorage(flatten.StorageCommandLine, flatten.NewPropertiesStorage(cmd), "cmd")
	l.AddStorage(flatten.StorageEnvironment, flatten.NewPropertiesStorage(env), "env")
	l.AddStorage(flatten.StorageDefault, flatten.NewPropertiesStorage(c.Properties), "")

	confDir, err := conf.Resolve(l, "${spring.app.config.dir:=./conf}")
	if err != nil {
		return nil, err
	}

	if err = loadFiles(l, confDir, nil); err != nil {
		return nil, err
	}

	// Profiles are designed to be orthogonal and independent.
	strActiveProfiles, err := conf.Resolve(l, "${spring.profiles.active:=}")
	if err != nil {
		return nil, err
	}
	activeProfiles := checkDuplicates(strings.Split(strActiveProfiles, ","))
	if len(activeProfiles) > 0 {
		if err = loadFiles(l, confDir, activeProfiles); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// checkDuplicates removes duplicate strings while preserving the
// original order. Empty items are ignored.
func checkDuplicates(arr []string) []string {
	var result []string
	temp := make(map[string]struct{})
	for _, s := range arr {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		if _, ok := temp[s]; ok {
			continue
		}
		result = append(result, s)
		temp[s] = struct{}{}
	}
	return result
}

// loadFiles loads candidate configuration files in order and adds them
// to the layered storage. File paths may contain property placeholders
// that are resolved before loading.
//
// Non-existent files are skipped, while other loading errors abort the process.
// Loaded files may declare additional imports via spring.app.imports.
func loadFiles(l *flatten.LayeredStorage, dir string, activeProfiles []string) error {
	extensions := []string{".properties", ".yaml", ".yml", ".toml", ".tml", ".json"}

	var files []string
	if activeProfiles == nil {
		for _, ext := range extensions {
			files = append(files, filepath.Join(dir, "app"+ext))
		}
	} else {
		for _, s := range activeProfiles {
			for _, ext := range extensions {
				files = append(files, filepath.Join(dir, "app-"+s+ext))
			}
		}
	}

	for _, s := range files {
		// Resolve property placeholders in the file name
		filename, err := conf.Resolve(l, s)
		if err != nil {
			return err
		}

		// Load the file
		p, err := conf.Load(filename)
		if err != nil {
			// Don't use `os.IsNotExist`
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}

		// Add the file to the layered storage
		if activeProfiles == nil {
			l.AddStorage(flatten.StorageAppFile, flatten.NewPropertiesStorage(p), filename)
		} else {
			l.AddStorage(flatten.StorageProfileFile, flatten.NewPropertiesStorage(p), filename)
		}

		// Load file imports; later-loaded sources override earlier ones
		if err = loadFileImports(l, p, activeProfiles); err != nil {
			return err
		}
	}
	return nil
}

// loadFileImports loads additional configuration files declared by
// the property `spring.app.imports`.
//
// Only one level of import is supported; imported files are not allowed
// to declare further imports.
func loadFileImports(l *flatten.LayeredStorage, p *flatten.Properties, activeProfiles []string) error {
	var i struct {
		Imports []string `value:"${spring.app.imports:=}"`
	}
	if err := conf.Bind(flatten.NewPropertiesStorage(p), &i); err != nil {
		return err
	}
	for _, source := range checkDuplicates(i.Imports) {
		str, err := conf.Resolve(l, source)
		if err != nil {
			return err
		}
		c, err := conf.Load(str)
		if err != nil {
			return err
		}
		if activeProfiles == nil {
			l.AddStorage(flatten.StorageAppFile, flatten.NewPropertiesStorage(c), str)
		} else {
			l.AddStorage(flatten.StorageProfileFile, flatten.NewPropertiesStorage(c), str)
		}
	}
	return nil
}
