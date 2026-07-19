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

package cmd

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go-spring.org/gs/cmd/k8s"
	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
)

// NewK8sCmd builds the `gs k8s` subcommand: it generates Kubernetes deploy
// scaffolding — a multi-stage Dockerfile, Kustomize base + overlays
// (Deployment/Service/HPA/ServiceMonitor), and a cloud-native config profile —
// into the current project. The output is an editable starting point, not a
// runtime dependency: probes wire to the actuator management port, preStop and
// the shutdown windows align with the framework's graceful-drain defaults, and
// logging switches to JSON-on-stdout under the k8s profile.
func NewK8sCmd() *cobra.Command {
	var port int
	var image string
	var force bool

	c := &cobra.Command{
		Use:          "k8s",
		Short:        "generate Kubernetes deploy scaffolding (Dockerfile, manifests)",
		Example:      "  gs k8s --port 9090",
		SilenceUsage: true,
	}
	c.Flags().IntVar(&port, "port", 9090, "primary application HTTP port exposed by the container")
	c.Flags().StringVar(&image, "image", "", "container image repository (default: project name)")
	c.Flags().BoolVar(&force, "force", false, "overwrite existing files instead of skipping them")
	runcmd.BindFlag(c)

	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runK8s(port, image, force)
	}
	return c
}

// runK8s renders the deploy scaffolding into the current working directory. It
// reads gs.json for the module path (reusing readProjectMeta), derives the
// Kubernetes resource/image name from the module leaf, and substitutes those
// plus the chosen ports into the embedded templates.
func runK8s(port int, image string, force bool) error {
	currDir, err := os.Getwd()
	if err != nil {
		return errutil.Explain(err, "get working directory")
	}

	meta, err := readProjectMeta(currDir)
	if err != nil {
		return err
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

	log.Printf("[INFO] Generating Kubernetes deploy scaffolding for %q", appName)
	if err := k8s.Write(currDir, replaces, force); err != nil {
		return err
	}

	log.Println("[INFO] Done. Next: build the image and apply an overlay, e.g.")
	log.Printf("[INFO]   docker build -t %s:latest .", image)
	log.Println("[INFO]   kubectl apply -k deploy/k8s/overlays/dev")
	return nil
}

// toDNS1123 lowercases s and replaces every character that is not a lowercase
// letter, digit, or hyphen with a hyphen, then trims leading/trailing hyphens,
// yielding a name usable as a Kubernetes object name and image tag. An empty
// result falls back to "app".
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
