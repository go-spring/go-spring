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

package StarterConfigFile

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register "configtree" as a second configuration provider on the SAME
	// controller singleton that backs "file-watch", so it shares the watch +
	// refresh bridge (ensureWatch / watchLoop / TriggerRefresh). A source such as
	//
	//	optional:configtree:/etc/config
	//
	// loads a directory tree at startup where each leaf file becomes one
	// property: its path relative to the root (segments joined by ".") is the
	// key and its unparsed, trimmed content is the value. This is the shape of a
	// Kubernetes Secret / env-style ConfigMap mount (many scalar key files), and
	// the model Spring Boot calls "configtree".
	conf.RegisterProvider("configtree", fileWatchController.LoadConfigTree)
}

// LoadConfigTree implements conf/provider.Provider for the "configtree" source.
// It walks a directory tree, turning each non-dot leaf file into a property
// keyed by its dotted relative path and valued by its trimmed raw content, and
// installs a watcher on every directory in the tree so any change triggers an
// application property refresh.
func (c *configFileController) LoadConfigTree(optional bool, source string) (map[string]string, error) {
	path := source
	if path == "" {
		return nil, errutil.Explain(nil, "configtree: missing path")
	}

	log.Debugf(context.Background(), starterTag, "loading configtree from %s", path)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) && optional {
			log.Warnf(context.Background(), starterTag, "optional configtree path %s not found (skipped)", path)
			return nil, nil
		}
		log.Errorf(context.Background(), starterTag, "stat %s failed: %v", path, err)
		return nil, errutil.Explain(err, "configtree: stat %s failed", path)
	}
	if !info.IsDir() {
		log.Errorf(context.Background(), starterTag, "configtree expects a directory, got file %s", path)
		return nil, errutil.Explain(nil, "configtree expects a directory, got file %s (a single config document belongs to the file-watch provider)", path)
	}

	m, err := walkConfigTree(path, c.ensureWatch)
	if err != nil {
		return nil, err
	}
	log.Infof(context.Background(), starterTag, "loaded configtree from dir=%s keys=%d", path, len(m))
	return m, nil
}

// walkConfigTree walks root recursively. Each leaf file whose name does not
// start with '.' contributes one entry to the result map:
//
//	key   = the file path relative to root, with path separators turned into "."
//	value = the file content with surrounding whitespace trimmed (NOT parsed)
//
// Entries whose name starts with '.' are skipped at every level — this excludes
// the Kubernetes projected-volume bookkeeping ("..data", the timestamped temp
// dirs) as well as dotfiles. The watch callback is invoked once for every
// (non-skipped) directory in the tree, including root, so a change anywhere
// under the tree can be observed. It is a pure function (apart from the watch
// callback and reading the filesystem) so it can be unit-tested without the IoC
// container.
func walkConfigTree(root string, watch func(dir string)) (map[string]string, error) {
	m := map[string]string{}
	err := filepath.WalkDir(root, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return errutil.Explain(err, "configtree: walk %s failed", full)
		}
		name := d.Name()
		// The root itself has no useful base name; never skip it. Everything else
		// whose name starts with '.' (..data, ..timestamp dirs, dotfiles) is skipped.
		if full != root && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if watch != nil {
				watch(full)
			}
			return nil
		}
		// Leaf file (a real file or a symlink to one, e.g. a K8s key symlink).
		b, readErr := os.ReadFile(full)
		if readErr != nil {
			return errutil.Explain(readErr, "configtree: read %s failed", full)
		}
		rel, relErr := filepath.Rel(root, full)
		if relErr != nil {
			return errutil.Explain(relErr, "configtree: relativize %s failed", full)
		}
		key := strings.ReplaceAll(filepath.ToSlash(rel), "/", ".")
		m[key] = strings.TrimSpace(string(b))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}
