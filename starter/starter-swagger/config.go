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

package StarterSwagger

// Config configures the Swagger UI documentation endpoint.
//
// The UI itself is not owned by this starter: its JavaScript/CSS assets are
// loaded from a CDN (AssetBaseURL) at runtime, so the starter ships no vendored
// megabytes and only serves a tiny HTML shell plus the generated OpenAPI spec.
type Config struct {
	// BasePath is the subtree the UI is mounted under. Its trailing slash is
	// normalized in NewUI so the endpoint claims the whole subtree
	// ("/swagger/", "/swagger/index.html", "/swagger/openapi.json").
	BasePath string `value:"${basePath:=/swagger}"`

	// SpecFile is the path to the OpenAPI document produced by
	// `gs-http-gen --openapi` (default openapi.json). It is read once at
	// startup so a missing or unreadable spec fails fast rather than 404ing
	// after the app is live.
	SpecFile string `value:"${specFile:=openapi.json}"`

	// Title is the browser tab title of the docs page.
	Title string `value:"${title:=API Documentation}"`

	// AssetBaseURL is the CDN base that serves swagger-ui-bundle.js and
	// swagger-ui.css. Pin a major version so a breaking UI release cannot
	// silently change the page. Point it at a self-hosted mirror in
	// air-gapped environments.
	AssetBaseURL string `value:"${assetBaseURL:=https://unpkg.com/swagger-ui-dist@5}"`
}
