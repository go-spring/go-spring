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
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"go-spring.org/gs/cmd/feature"
)

// bindFeatureFlags registers one boolean flag per manifest feature on c (the
// flag name IS the feature key) and returns a collector that reports which
// features the user selected. It is shared by `gs init` (which prunes the
// unselected) and `gs add` (which copies the selected), so both expose the
// same feature vocabulary.
//
// Features carry no per-flag parameters today, so a plain bool suffices; if
// structural params return (see feature.ParseParams) this becomes a value flag.
func bindFeatureFlags(c *cobra.Command, m *feature.Manifest) func() map[string]struct{} {
	for _, f := range m.Features {
		c.Flags().Bool(f.Key, false, f.Desc)
	}
	return func() map[string]struct{} {
		sel := make(map[string]struct{})
		for _, f := range m.Features {
			if v, _ := c.Flags().GetBool(f.Key); v {
				sel[f.Key] = struct{}{}
			}
		}
		return sel
	}
}

// formatFeatures renders the manifest grouped by category, for `--list-features`.
func formatFeatures(m *feature.Manifest) string {
	byCat := map[string][]feature.Feature{}
	var cats []string
	for _, f := range m.Features {
		cat := f.Category
		if cat == "" {
			cat = "other"
		}
		if _, seen := byCat[cat]; !seen {
			cats = append(cats, cat)
		}
		byCat[cat] = append(byCat[cat], f)
	}
	sort.Strings(cats)

	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	for _, cat := range cats {
		fmt.Fprintf(tw, "%s:\n", cat)
		fs := byCat[cat]
		sort.Slice(fs, func(i, j int) bool { return fs[i].Key < fs[j].Key })
		for _, f := range fs {
			fmt.Fprintf(tw, "  --%s\t%s\n", f.Key, f.Desc)
		}
	}
	_ = tw.Flush()
	return sb.String()
}
