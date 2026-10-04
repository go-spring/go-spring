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

// This file implements the "file-watch" provider: a single hot-reloadable
// configuration document. Each import accepts exactly one file, parsed by
// extension (delegated to the shared conf reader registry). Use it for a whole
// configuration document — e.g. a Kubernetes ConfigMap/Secret key that carries
// an application.yaml. For layered overrides, declare one file-watch import per
// file in priority order (later imports win), the same way spring.config.import
// composes every other source. file-watch does NOT merge a directory — use the
// sibling "configtree" provider for a directory of scalar key files.

package StarterConfigFile

import (
	"context"
	"os"
	"path/filepath"

	"go-spring.org/log"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// fileWatchCtrl is the "file-watch" provider: one configuration document per
// import, parsed by extension. It embeds watchCore for the shared watch +
// refresh bridge.
type fileWatchCtrl struct {
	watchCore
}

// newFileWatchCtrl creates the "file-watch" provider controller with its
// watch machinery ready.
func newFileWatchCtrl() *fileWatchCtrl {
	return &fileWatchCtrl{watchCore: newWatchCore()}
}

// Load implements conf/provider.Provider. It reads a single configuration file
// (format detected by extension via the shared conf reader registry) and
// installs a watcher on its parent directory that triggers an application
// property refresh on change.
func (c *fileWatchCtrl) Load(optional bool, source string) (map[string]string, error) {
	// Load is the top of this chain and Provider.Load takes no context, so the
	// one ctx the whole load shares is minted here — ensureWatch below receives
	// it instead of minting its own.
	ctx := context.Background()

	path := source
	if path == "" {
		return nil, errutil.Explain(nil, "file-watch: missing path")
	}

	log.Debug(ctx, starterTag, func() []log.Field {
		return []log.Field{
			log.String("path", path),
			log.Msg("loading file-watch config"),
		}
	})

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) && optional {
			log.Warn(ctx, starterTag,
				log.String("path", path),
				log.Msg("optional config path not found, skipped"))
			return nil, nil
		}
		log.Error(ctx, starterTag,
			log.String("path", path),
			log.Err(err),
			log.Msg("stat failed"))
		return nil, errutil.Explain(err, "file-watch: stat %s failed", path)
	}

	if info.IsDir() {
		log.Error(ctx, starterTag,
			log.String("path", path),
			log.Msg("file-watch expects a single file, got a directory"))
		return nil, errutil.Explain(nil, "file-watch expects a single file, got directory %s (a directory of scalar key files belongs to the configtree provider)", path)
	}

	// Watch the parent directory, never the file itself: Kubernetes updates a
	// ConfigMap/Secret mount by writing a fresh timestamped directory and
	// atomically renaming the "..data" symlink, and common editors save by
	// atomic rename too. In both cases the file's inode changes, so a per-file
	// inotify watch would be left on a stale inode after the first update.
	// Watching the directory keeps hot-reload working across the swap.
	c.ensureWatch(ctx, filepath.Dir(path))

	// Format is detected by extension through the shared conf reader registry,
	// so there is no per-provider format map to maintain.
	parsed, err := reader.ReadFile(path)
	if err != nil {
		return nil, errutil.Explain(err, "file-watch: read %s failed", path)
	}
	m := flatten.Flatten(parsed)

	log.Info(ctx, starterTag,
		log.String("path", path),
		log.Int("keys", len(m)),
		log.Msg("loaded file-watch config"))
	return m, nil
}
