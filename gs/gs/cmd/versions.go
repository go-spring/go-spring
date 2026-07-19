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
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"go-spring.org/gs/internal/bom"
	"go-spring.org/gs/internal/runcmd"
	"go-spring.org/stdlib/errutil"
)

// baselineFile is the name of the version manifest at the repo root.
const baselineFile = "versions.yaml"

// NewVersionsCmd builds the `gs versions` subcommand: BOM-style dependency
// version governance. It reads versions.yaml at the repo root (the blessed
// third-party versions) and scans every go.mod under go.work to report — and,
// on request, align — modules that drift from that baseline.
//
// The default paths (check, diff) are read-only so the command is safe to run
// in CI and never conflicts with concurrent module work; only `apply` writes,
// and only to a single named module at a time.
func NewVersionsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:          "versions",
		Short:        "check module dependency versions against versions.yaml (BOM governance)",
		Example:      "  gs versions check\n  gs versions diff\n  gs versions apply starter/starter-config-etcd",
		SilenceUsage: true,
	}
	c.AddCommand(newVersionsCheckCmd())
	c.AddCommand(newVersionsDiffCmd())
	c.AddCommand(newVersionsApplyCmd())
	return c
}

// loadRootBaseline finds the repo root from the cwd, loads versions.yaml, and
// returns both. Shared by all three subcommands.
func loadRootBaseline() (root string, base *bom.Baseline, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, errutil.Explain(err, "get working directory")
	}
	root, err = bom.FindRoot(cwd)
	if err != nil {
		return "", nil, err
	}
	base, err = bom.LoadBaseline(filepath.Join(root, baselineFile))
	if err != nil {
		return "", nil, err
	}
	return root, base, nil
}

func newVersionsCheckCmd() *cobra.Command {
	c := &cobra.Command{
		Use:          "check",
		Short:        "report modules whose dependency versions drift from the baseline (non-zero exit on drift)",
		SilenceUsage: true,
	}
	runcmd.BindFlag(c)
	c.RunE = func(_ *cobra.Command, _ []string) error {
		root, base, err := loadRootBaseline()
		if err != nil {
			return err
		}
		drifts, err := bom.Check(root, base)
		if err != nil {
			return err
		}
		if len(drifts) == 0 {
			fmt.Println("[INFO] all governed modules match versions.yaml")
			return nil
		}
		printDriftTable(drifts)
		fmt.Printf("\n[ERROR] %d version drift(s) from versions.yaml\n", len(drifts))
		// Non-zero exit for CI. SilenceUsage keeps cobra from printing usage.
		return errutil.Explain(fmt.Errorf("version drift detected"), "gs versions check")
	}
	return c
}

func newVersionsDiffCmd() *cobra.Command {
	c := &cobra.Command{
		Use:          "diff",
		Short:        "show per-dependency detail of how modules deviate from the baseline",
		SilenceUsage: true,
	}
	runcmd.BindFlag(c)
	c.RunE = func(_ *cobra.Command, _ []string) error {
		root, base, err := loadRootBaseline()
		if err != nil {
			return err
		}
		drifts, err := bom.Check(root, base)
		if err != nil {
			return err
		}
		if len(drifts) == 0 {
			fmt.Println("[INFO] no deviations from versions.yaml")
			return nil
		}
		printDiffByDep(drifts)
		return nil
	}
	return c
}

func newVersionsApplyCmd() *cobra.Command {
	c := &cobra.Command{
		Use:          "apply <module>",
		Short:        "align a single module's go.mod to the baseline (writes one go.mod)",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}
	runcmd.BindFlag(c)
	c.RunE = func(_ *cobra.Command, args []string) error {
		root, base, err := loadRootBaseline()
		if err != nil {
			return err
		}
		changes, err := bom.Apply(root, base, args[0])
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			fmt.Printf("[INFO] %s already matches versions.yaml\n", args[0])
			return nil
		}
		for _, d := range changes {
			fmt.Printf("[INFO] %s: %s %s -> %s\n", d.Dir, d.Dep, d.Found, d.Baseline)
		}
		fmt.Printf("[INFO] aligned %d require(s) in %s; run `gs go mod tidy` in that module to settle go.sum\n",
			len(changes), changes[0].Dir)
		return nil
	}
	return c
}

// printDriftTable renders the drift list as an aligned table, sorted by
// directory then dependency for stable output.
func printDriftTable(drifts []bom.Drift) {
	sortDrifts(drifts)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() { _ = tw.Flush() }()
	fmt.Fprintln(tw, "MODULE\tDEPENDENCY\tFOUND\tBASELINE\tDRIFT")
	for _, d := range drifts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", d.Dir, d.Dep, d.Found, d.Baseline, d.Kind)
	}
}

// printDiffByDep groups deviations by dependency, showing the blessed version
// once and the deviating modules beneath it — the view for human remediation
// decisions.
func printDiffByDep(drifts []bom.Drift) {
	byDep := map[string][]bom.Drift{}
	for _, d := range drifts {
		byDep[d.Dep] = append(byDep[d.Dep], d)
	}
	deps := make([]string, 0, len(byDep))
	for dep := range byDep {
		deps = append(deps, dep)
	}
	sort.Strings(deps)

	for _, dep := range deps {
		items := byDep[dep]
		fmt.Printf("%s (baseline %s)\n", dep, items[0].Baseline)
		sort.Slice(items, func(i, j int) bool { return items[i].Dir < items[j].Dir })
		for _, d := range items {
			suffix := ""
			if d.Indirect {
				suffix = " (indirect)"
			}
			fmt.Printf("  %-7s %s @ %s%s\n", string(d.Kind), d.Dir, d.Found, suffix)
		}
	}
}

// sortDrifts orders drifts by directory then dependency for deterministic output.
func sortDrifts(drifts []bom.Drift) {
	sort.Slice(drifts, func(i, j int) bool {
		if drifts[i].Dir != drifts[j].Dir {
			return drifts[i].Dir < drifts[j].Dir
		}
		return drifts[i].Dep < drifts[j].Dep
	})
}
