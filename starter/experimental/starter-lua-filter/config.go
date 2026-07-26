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

package StarterLuaFilter

// Config defines a single Lua filter instance. A filter is a Lua script that
// runs at the net/http layer on every request — the same position an Envoy or
// OpenResty Lua filter occupies in a gateway data plane — so it stays agnostic
// to whichever web framework (gin/echo/hertz/net-http) sits behind it.
type Config struct {
	// Script is the path to the Lua source file, resolved relative to the
	// application working directory.
	Script string `value:"${script}"`
}
