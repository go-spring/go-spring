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

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const Version = "v0.2.4"

// execDir is the directory where the executable is located.
var execDir string

func init() {
	filename, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to determine executable path: %s\n", err.Error())
		os.Exit(1)
	}
	execDir = filepath.Dir(filename)
}

func main() {
	if len(os.Args) <= 1 ||
		os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" ||
		os.Args[1] == "-v" || os.Args[1] == "--version" {
		showHelp()
		os.Exit(0)
	}

	// Call the requested tool with provided arguments
	callTool("gs-"+os.Args[1], os.Args[2:]...)
}

// showHelp displays the help information,
// including available tools and usage instructions.
func showHelp() {
	tools := scanTools()
	fmt.Printf("Go-Spring Toolkit Manager %s.\n", Version)
	fmt.Println()
	fmt.Println("The toolkit manager looks for executable files prefixed with `gs-` in its directory\n" +
		"(usually `$GOPATH/bin`) and manages them as available tools.")
	fmt.Println()
	fmt.Println("When a user invokes a tool, the toolkit manager executes the corresponding executable\n" +
		"and passing the arguments.")
	fmt.Println()
	fmt.Println("Available tools:")

	if len(tools) == 0 {
		fmt.Println("  No tools found. Please make sure tools with 'gs-' prefix are in the same directory.")
	} else {
		var maxLen int
		for _, tool := range tools {
			if len(tool) > maxLen {
				maxLen = len(tool)
			}
		}

		for _, tool := range tools {
			var sb strings.Builder
			sb.WriteString("  ")
			toolName := strings.TrimPrefix(tool, "gs-")
			sb.WriteString(toolName)
			sb.WriteString(strings.Repeat(" ", maxLen-len(toolName)-1))

			if version, desc, err := getToolInfo(tool); err != nil {
				sb.WriteString("Failed to get info: ")
				sb.WriteString(err.Error())
			} else {
				sb.WriteString("(")
				sb.WriteString(version)
				sb.WriteString(") ")
				sb.WriteString(desc)
			}
			fmt.Println(sb.String())
		}
	}

	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gs <tool> --help  Show tool's help information")
	fmt.Println("  gs <tool> [args]  Call the tool with the arguments")
	fmt.Println()
	fmt.Println("Additional help topics:")
	fmt.Println("  gs --help     Show this help information")
	fmt.Println("  gs --version  Show this help information")
}

// scanTools scans the executable directory for tools with the 'gs-' prefix.
func scanTools() []string {
	entries, err := os.ReadDir(execDir)
	if err != nil {
		fmt.Printf("Error reading directory %s: %s\n", execDir, err.Error())
		os.Exit(1)
	}

	var tools []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "gs-") {
			tools = append(tools, entry.Name())
		}
	}
	slices.Sort(tools)
	return tools
}

// callTool executes the specified tool with the given arguments.
func callTool(tool string, args ...string) {
	if !slices.Contains(scanTools(), tool) {
		fmt.Printf("Error: tool '%s' not found\n", strings.TrimPrefix(tool, "gs-"))
		fmt.Println("Run 'gs --help' to see a list of available tools.")
		os.Exit(1)
	}

	// Execute the tool with the provided arguments
	toolPath := filepath.Join(execDir, tool)
	cmd := exec.Command(toolPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running tool '%s': %s\n", tool, err.Error())
		if len(output) > 0 {
			fmt.Printf("Output: %s\n", string(output))
		}
		os.Exit(1)
	}
	fmt.Println(string(output))
}

// getToolInfo gets tool information,
// requiring the tool to print usage description on the first line
// and version on the second line when --version is used.
func getToolInfo(tool string) (version string, desc string, err error) {
	toolPath := filepath.Join(execDir, tool)
	cmd := exec.Command(toolPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("%w: [output] %s", err, string(output))
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return "", "", fmt.Errorf("invalid output: %s", string(output))
	}
	return lines[1], lines[0], nil
}
