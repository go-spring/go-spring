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

import "go-spring.org/spring/conf"

func init() {
	// Register "configtree" as a configuration provider backed by its own
	// configTreeCtrl (which embeds the same watch + refresh machinery as
	// "file-watch" via watchCore). A source such as
	//
	//	optional:configtree:/etc/config
	//
	// loads a directory tree at startup where each leaf file becomes one
	// property: its path relative to the root (segments joined by ".") is the
	// key and its unparsed, trimmed content is the value. This is the shape of a
	// Kubernetes Secret / env-style ConfigMap mount (many scalar key files), and
	// the model Spring Boot calls "configtree".
	//
	// The controller itself is registered (not its Load method): the runtime
	// holds it so its Close can stop the watchers at shutdown.
	conf.RegisterProvider("configtree", newConfigTreeCtrl())

	// Register "file-watch" as a configuration provider so a spring.config.import
	// entry such as
	//
	//	optional:file-watch:/etc/config/application.yaml
	//
	// loads a single file at startup and, whenever it changes, triggers a full
	// property refresh via the gs.RefreshProperties facade — no separate hook
	// wiring needed.
	conf.RegisterProvider("file-watch", newFileWatchCtrl())
}
