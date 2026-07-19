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

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
)

// pageTemplate is the minimal Swagger UI shell. The heavy assets (CSS + JS
// bundle) are pulled from AssetBaseURL at runtime; SwaggerUIBundle points at the
// spec served by this same endpoint so the whole doc set is same-origin.
var pageTemplate = template.Must(template.New("swagger").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>{{.Title}}</title>
  <link rel="stylesheet" href="{{.AssetBaseURL}}/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="{{.AssetBaseURL}}/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: "{{.SpecURL}}",
        dom_id: "#swagger-ui",
        deepLinking: true
      });
    };
  </script>
</body>
</html>`))

// UI is a self-contained Swagger UI [endpoint.Endpoint]. It owns the whole
// BasePath subtree and serves three things: the HTML shell at BasePath and
// BasePath+"/index.html", and the OpenAPI document at BasePath+"/openapi.json".
//
// It implements endpoint.Endpoint so the actuator auto-mounts it on the
// management port with no wiring; it is also a plain http.Handler, so an app
// without the actuator can mount it on its own HTTP server via *gs.HttpServeMux.
type UI struct {
	basePath string
	specURL  string
	spec     []byte
	page     []byte
}

// NewUI builds a UI from Config, reading the OpenAPI spec once so a missing or
// unreadable file fails fast at startup rather than after the server is live.
func NewUI(cfg Config) (*UI, error) {
	base := "/" + strings.Trim(cfg.BasePath, "/")

	spec, err := os.ReadFile(cfg.SpecFile)
	if err != nil {
		return nil, fmt.Errorf("swagger: reading spec file %q: %w", cfg.SpecFile, err)
	}

	specURL := base + "/openapi.json"
	var buf strings.Builder
	if err = pageTemplate.Execute(&buf, map[string]any{
		"Title":        cfg.Title,
		"AssetBaseURL": strings.TrimRight(cfg.AssetBaseURL, "/"),
		"SpecURL":      specURL,
	}); err != nil {
		return nil, fmt.Errorf("swagger: rendering page: %w", err)
	}

	return &UI{
		basePath: base,
		specURL:  specURL,
		spec:     spec,
		page:     []byte(buf.String()),
	}, nil
}

// Path returns the subtree the UI claims. The trailing slash makes the actuator
// mux route every request under BasePath (index page + spec) to this handler.
func (u *UI) Path() string { return u.basePath + "/" }

func (u *UI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case u.specURL:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(u.spec)
	case u.basePath, u.basePath + "/", u.basePath + "/index.html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(u.page)
	default:
		http.NotFound(w, r)
	}
}
