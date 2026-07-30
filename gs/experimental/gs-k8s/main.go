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

// Command gs-k8s is an external gs tool that generates Kubernetes deploy
// scaffolding — a multi-stage Dockerfile, Kustomize base + overlays
// (Deployment/Service/HPA/ServiceMonitor), and a cloud-native config profile —
// into the current project. The output is an editable starting point, not a
// runtime dependency.
//
// It follows the gs external-tool protocol (see gs/gs/tool/tool.go): the
// binary is named "gs-k8s", lives next to the gs binary, and prints a
// two-line description/version pair for `gs-k8s --version`.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const ToolVersion = "v0.0.1"

const toolDesc = "Generate Kubernetes deploy scaffolding (Dockerfile, manifests)."

// verbosity is set by -v flags peeled from args.
var verbosity int

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "--version" {
		fmt.Println(toolDesc)
		fmt.Println(ToolVersion)
		return
	}

	log.SetFlags(log.Ltime)

	var port int
	var image string
	var force bool
	var format string

	args := os.Args[1:]
	args = peelVerbosity(args)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			i++
			if i < len(args) {
				port, _ = strconv.Atoi(args[i])
			}
		case "--image":
			i++
			if i < len(args) {
				image = args[i]
			}
		case "--format":
			i++
			if i < len(args) {
				format = args[i]
			}
		case "--force":
			force = true
		case "--verbose":
			// already consumed by peelVerbosity
		default:
			// skip unknown flags
		}
	}

	if port == 0 {
		port = 9090
	}
	if format == "" {
		format = FormatKustomize
	}

	switch format {
	case FormatKustomize, FormatHelm:
	default:
		log.Fatalf("gs-k8s: unknown format %q (must be %q or %q)", format, FormatKustomize, FormatHelm)
	}

	currDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("gs-k8s: get working directory: %v", err)
	}

	meta, err := readProjectMeta(currDir)
	if err != nil {
		log.Fatalf("gs-k8s: %v", err)
	}

	appName := toDNS1123(moduleLeaf(meta.Module))
	if image == "" {
		image = appName
	}

	replaces := map[string]string{
		"GS_PROJECT_MODULE": meta.Module,
		"GS_PROJECT_NAME":   toPascal(moduleLeaf(meta.Module)),
		"GS_APP_NAME":       appName,
		"GS_APP_PORT":       strconv.Itoa(port),
		"GS_MGMT_PORT":      "9370",
		"GS_IMAGE":          image,
	}

	log.Printf("[INFO] Generating Kubernetes deploy scaffolding (%s) for %q", format, appName)
	if err := Write(currDir, replaces, force, format); err != nil {
		log.Fatalf("gs-k8s: %v", err)
	}

	log.Println("[INFO] Done. Next: build the image and apply the manifests, e.g.")
	log.Printf("[INFO]   docker build -t %s:latest .", image)
	if format == FormatHelm {
		log.Println("[INFO]   helm template deploy/helm | kubectl apply -f -")
	} else {
		log.Println("[INFO]   kubectl apply -k deploy/k8s/overlays/dev")
	}
}

// projectMeta mirrors the gs.json written at `gs init`.
type projectMeta struct {
	Module        string `json:"module"`
	Lang          string `json:"lang"`
	LayoutVersion string `json:"layout_version"`
}

// readProjectMeta loads gs.json from dir.
func readProjectMeta(dir string) (projectMeta, error) {
	var meta projectMeta
	b, err := os.ReadFile(filepath.Join(dir, "gs.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return meta, fmt.Errorf("gs.json not found: run `gs k8s` from a project root")
		}
		return meta, fmt.Errorf("read gs.json: %w", err)
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return meta, fmt.Errorf("parse gs.json: %w", err)
	}
	if meta.Module == "" {
		return meta, fmt.Errorf("gs.json is missing the module path")
	}
	return meta, nil
}

// moduleLeaf returns the final path segment of a Go module path.
func moduleLeaf(module string) string {
	ss := strings.Split(module, "/")
	return ss[len(ss)-1]
}

// toPascal converts a name in snake_case or kebab-case to PascalCase.
func toPascal(s string) string {
	var sb strings.Builder
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-'
	}) {
		c := part[0]
		if 'a' <= c && c <= 'z' {
			c = c - 'a' + 'A'
		}
		sb.WriteByte(c)
		if len(part) > 1 {
			sb.WriteString(part[1:])
		}
	}
	return sb.String()
}

// toDNS1123 lowercases s and replaces every character that is not a lowercase
// letter, digit, or hyphen with a hyphen, then trims leading/trailing hyphens.
func toDNS1123(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "app"
	}
	return out
}

// peelVerbosity consumes leading verbosity flags from args and returns remaining.
func peelVerbosity(args []string) []string {
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--verbose":
			verbosity++
		case strings.HasPrefix(a, "--verbose="):
			n, err := strconv.Atoi(a[len("--verbose="):])
			if err != nil {
				return args[i:]
			}
			verbosity += n
		case len(a) >= 2 && a[0] == '-' && strings.Trim(a[1:], "v") == "":
			verbosity += len(a) - 1
		default:
			return args[i:]
		}
	}
	return args[i:]
}
