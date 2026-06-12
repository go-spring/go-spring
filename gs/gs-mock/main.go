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
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-spring/gs-mock/gsmock"
)

// stdOut is the writer used for outputting the generated code.
// By default, it writes to os.Stdout,
// but can be overridden for testing or redirection.
var stdOut io.Writer = os.Stdout

// ToolVersion specifies the version of this mock generation tool.
const ToolVersion = "v0.0.8"

// flags holds the command-line flag values for output file and interface selection.
var flags struct {
	OutputFile     string // Path to the output Go file for generated mocks.
	MockInterfaces string // Comma-separated list of interface names to mock.
}

func init() {
	flag.StringVar(&flags.OutputFile, "o", "", "Path to the output Go file. Defaults to stdout if not specified.")
	flag.StringVar(&flags.OutputFile, "output", "", "Alias for -o. Specifies the output file path for generated mocks.")
	flag.StringVar(&flags.MockInterfaces, "i", "", "Comma-separated list of interface names to mock (e.g., 'Reader,Writer'). Prefix with '!' to exclude specific interfaces (e.g., '!Logger'). Defaults to mocking all interfaces.")
	flag.StringVar(&flags.MockInterfaces, "interfaces", "", "Alias for -i. Specifies interfaces to include or exclude for mocking. Use '!' prefix for exclusions.")
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("A tool used to generate Go mock code.")
		fmt.Println(ToolVersion)
		return
	}
	flag.Parse()
	run(runConfig{
		SourceDir:      ".",
		OutputFile:     flags.OutputFile,
		MockInterfaces: flags.MockInterfaces,
	})
}

// runConfig holds configuration parameters for the generator.
type runConfig struct {
	SourceDir      string // Directory containing source Go files to scan.
	OutputFile     string // Path to output Go file for generated mocks.
	MockInterfaces string // Comma-separated interface filter string.
}

// run executes the main logic of scanning interfaces and generating mocks.
func run(param runConfig) {
	ctx := scanContext{
		OutputFile:        param.OutputFile,
		IncludeInterfaces: make(map[string]struct{}),
		ExcludeInterfaces: make(map[string]struct{}),
	}

	// Parse interface filters
	if s := param.MockInterfaces; len(s) > 0 {
		if s[0] == '\'' || s[0] == '"' {
			param.MockInterfaces = s[1 : len(s)-1] // Remove surrounding quotes
		}
		ctx.parse(param.MockInterfaces)
	}

	// Map of import path => package name to detect conflicts
	pkgMap := make(map[string]string)
	interfaces := scanDir(param.SourceDir, ctx, pkgMap)

	// Collect necessary imports for generated mocks
	imports := make(map[string]string)
	imports["gsmock"] = "github.com/go-spring/gs-mock/gsmock"
	for _, m := range interfaces {
		maps.Copy(imports, m.Imports)
	}

	s := bytes.NewBuffer(nil)

	// Generate import statements
	h := bytes.NewBuffer(nil)
	for pkgName, pkgPath := range imports {
		ss := strings.Split(pkgPath, "/")
		if pkgName == ss[len(ss)-1] {
			_, _ = fmt.Fprintf(h, "\t\"%s\"\n", pkgPath)
		} else {
			_, _ = fmt.Fprintf(h, "\t%s \"%s\"\n", pkgName, pkgPath)
		}
	}

	// Build the command string for documentation
	var toolCommand string
	if len(param.OutputFile) > 0 {
		toolCommand += "-o " + param.OutputFile
	}
	if len(param.MockInterfaces) > 0 {
		toolCommand += " -i '" + param.MockInterfaces + "'"
	}

	packageName := interfaces[0].Package

	// Execute file header template
	if err := tmplFileHeader.Execute(s, map[string]any{
		"ToolVersion": ToolVersion,
		"ToolCommand": toolCommand,
		"Package":     packageName,
		"Imports":     h.String(),
	}); err != nil {
		panic(fmt.Errorf("error executing template(header): %w", err))
	}

	// Generate code for each interface and its methods
	for _, i := range interfaces {
		if err := tmplInterface.Execute(s, i); err != nil {
			panic(fmt.Errorf("error executing template(interface#%s): %w", i.Name, err))
		}
		for _, m := range i.Methods {
			if err := tmplMethod.Execute(s, map[string]any{
				"i": i,
				"m": m,
			}); err != nil {
				panic(fmt.Errorf("error executing template(method#%s): %w", m.Name, err))
			}
		}
	}

	// Format the generated source code
	b, err := format.Source(s.Bytes())
	if err != nil {
		panic(fmt.Errorf("error formatting source code: %w", err))
	}

	// Output generated code to file or stdout
	switch param.OutputFile {
	case "":
		if _, err = stdOut.Write(b); err != nil {
			panic(fmt.Errorf("error writing to stdout: %w", err))
		}
	default:
		outputFile := filepath.Join(param.SourceDir, param.OutputFile)
		if err = os.WriteFile(outputFile, b, os.ModePerm); err != nil {
			panic(fmt.Errorf("error writing to file(%s): %w", outputFile, err))
		}
	}
}

// scanContext holds state and filters during interface scanning.
type scanContext struct {
	OutputFile        string
	IncludeInterfaces map[string]struct{}
	ExcludeInterfaces map[string]struct{}
}

// parse converts the comma-separated interface filter string into inclusion/exclusion maps.
func (ctx *scanContext) parse(mockInterfaces string) {
	if len(mockInterfaces) == 0 {
		return
	}
	for s := range strings.SplitSeq(mockInterfaces, ",") {
		if s = strings.TrimSpace(s); len(s) == 0 {
			continue
		}
		if s[0] == '!' {
			ctx.ExcludeInterfaces[strings.TrimSpace(s[1:])] = struct{}{}
		} else {
			ctx.IncludeInterfaces[strings.TrimSpace(s)] = struct{}{}
		}
	}
}

// mock determines whether a given interface name should be mocked.
func (ctx *scanContext) mock(name string) bool {
	if len(ctx.IncludeInterfaces) > 0 {
		_, ok := ctx.IncludeInterfaces[name]
		return ok
	}
	_, ok := ctx.ExcludeInterfaces[name]
	return !ok
}

// Interface describes a mockable interface.
type Interface struct {
	Package         string            // Package name where the interface resides
	Name            string            // Interface name
	TypeParams      string            // Generic type parameters (e.g., "T any")
	TypeParamNames  string            // Generic type names only (e.g., "T")
	EmbedInterfaces string            // Embedded interfaces as string
	Methods         []Method          // Methods in the interface
	File            string            // Source file path
	Imports         map[string]string // Required imports for this interface
}

// Method describes a single method within an interface.
type Method struct {
	Name            string // Method name
	VariadicFlag    string // "Var" if the method has variadic parameters
	Params          string // Method parameters as string (e.g., "a int, b string")
	ParamNames      string // Comma-separated parameter names only
	ParamCount      int    // Number of parameters
	ResultTypes     string // Return types as a string (e.g., "(int, error)")
	ResultTmplTypes string // Return types for template generation (e.g., "[int, error]")
	ResultCount     int    // Number of return values
	MockerTmplTypes string // Full template type parameters for the mocker
}

// scanDir scans the given directory for Go files and returns all interfaces to be mocked.
func scanDir(dir string, ctx scanContext, pkgs map[string]string) []Interface {
	entries, err := os.ReadDir(dir)
	if err != nil {
		panic(fmt.Errorf("error reading directory: %w", err))
	}
	var ret []Interface
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		if entry.Name() == ctx.OutputFile {
			continue
		}
		arr := scanFile(ctx, filepath.Join(dir, entry.Name()), pkgs)
		ret = append(ret, arr...)
	}
	return ret
}

// scanFile parses a Go source file and extracts all mockable interfaces.
func scanFile(ctx scanContext, file string, pkgs map[string]string) []Interface {
	mode := parser.AllErrors
	node, err := parser.ParseFile(token.NewFileSet(), file, nil, mode)
	if err != nil {
		panic(fmt.Errorf("error parsing file(%s): %w", file, err))
	}

	needImports := make(map[string]string) // Imports needed for this file
	totalImports := make(map[string]string)

	// Collect package imports
	for _, spec := range node.Imports {
		pkgPath := strings.Trim(spec.Path.Value, "\"")

		var pkgName string
		if spec.Name != nil {
			pkgName = spec.Name.Name
		} else {
			ss := strings.Split(pkgPath, "/")
			pkgName = ss[len(ss)-1]
		}

		// Detect import conflicts
		if v, ok := pkgs[pkgPath]; ok && v != pkgName {
			panic(fmt.Sprintf("import package name conflict: %s, %s", v, pkgName))
		}
		pkgs[pkgPath] = pkgName
		totalImports[pkgName] = pkgPath
	}

	putImport := func(pkgNames []string) {
		for _, s := range pkgNames {
			pkgName := s[:len(s)-1] // Remove trailing dot
			if pkgPath, ok := totalImports[pkgName]; ok {
				needImports[pkgName] = pkgPath
			}
		}
	}

	var ret []Interface
	for _, decl := range node.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok || d.Tok != token.TYPE {
			continue
		}

		for _, spec := range d.Specs {
			s := spec.(*ast.TypeSpec)
			t, ok := s.Type.(*ast.InterfaceType)
			if !ok || len(t.Methods.List) == 0 {
				continue
			}

			name := s.Name.String()
			if !ctx.mock(name) {
				continue
			}

			// Collect type parameters
			var (
				typeParamArray     []string
				typeParamNameArray []string
			)
			if s.TypeParams != nil {
				for _, f := range s.TypeParams.List {
					fName := f.Names[0].Name
					typeText, pkgNames := getTypeText(f.Type)
					typeParamArray = append(typeParamArray, fName+" "+typeText)
					typeParamNameArray = append(typeParamNameArray, fName)
					putImport(pkgNames)
				}
			}

			// Collect embedded interfaces
			var embedInterfaces strings.Builder
			for _, method := range t.Methods.List {
				if len(method.Names) == 0 {
					embedInterfaces.WriteString("\t")
					typeText, pkgNames := getTypeText(method.Type)
					embedInterfaces.WriteString(typeText)
					embedInterfaces.WriteString("\n")
					putImport(pkgNames)
				}
			}

			// Collect methods
			var methods []Method
			for _, method := range t.Methods.List {
				if len(method.Names) == 0 {
					continue
				}
				ft := method.Type.(*ast.FuncType)
				methodName := method.Names[0].Name

				paramCount := 0
				resultCount := 0

				var (
					varText    string
					params     []string
					paramNames []string
					paramTypes []string
				)
				if ft.Params != nil {
					for _, param := range ft.Params.List {
						var tempNames []string
						if len(param.Names) == 0 {
							tempNames = append(tempNames, "r"+strconv.Itoa(paramCount))
						} else {
							for _, r := range param.Names {
								tempNames = append(tempNames, r.Name)
							}
						}

						typeText, pkgNames := getTypeText(param.Type)
						for _, paramName := range tempNames {
							if strings.HasPrefix(typeText, "...") {
								varText = "Var"
								paramTypes = append(paramTypes, typeText[3:])
							} else {
								paramTypes = append(paramTypes, typeText)
							}
							paramNames = append(paramNames, paramName)
							params = append(params, paramName+" "+typeText)
						}
						putImport(pkgNames)
						paramCount += len(tempNames)
					}
				}

				if N := gsmock.MaxParamCount - 1; paramCount > N {
					panic(fmt.Sprintf("have more than %d parameters", N))
				}

				var resultTypeArray []string
				if ft.Results != nil {
					for _, result := range ft.Results.List {
						var tempNames []string
						if len(result.Names) == 0 {
							tempNames = append(tempNames, "r"+strconv.Itoa(resultCount))
						} else {
							for _, r := range result.Names {
								tempNames = append(tempNames, r.Name)
							}
						}

						typeText, pkgNames := getTypeText(result.Type)
						for range tempNames {
							resultTypeArray = append(resultTypeArray, typeText)
						}
						putImport(pkgNames)
						resultCount += len(tempNames)
					}
				}

				if resultCount > gsmock.MaxResultCount {
					panic(fmt.Sprintf("have more than %d results", gsmock.MaxResultCount))
				}

				mockerTmplTypes := ""
				if len(paramTypes) > 0 || len(resultTypeArray) > 0 {
					mockerTmplTypes += strings.Join(paramTypes, ", ")
					if mockerTmplTypes != "" {
						mockerTmplTypes += ", "
					}
					mockerTmplTypes += strings.Join(resultTypeArray, ", ")
					mockerTmplTypes = "[" + mockerTmplTypes + "]"
				}

				resultTypes := ""
				resultTmplTypes := ""
				if len(resultTypeArray) > 0 {
					resultTypes = "(" + strings.Join(resultTypeArray, ", ") + ")"
					resultTmplTypes = "[" + strings.Join(resultTypeArray, ", ") + "]"
				}

				methods = append(methods, Method{
					Name:            methodName,
					VariadicFlag:    varText,
					Params:          strings.Join(params, ", "),
					ParamNames:      strings.Join(paramNames, ", "),
					ParamCount:      paramCount,
					ResultTypes:     resultTypes,
					ResultTmplTypes: resultTmplTypes,
					ResultCount:     resultCount,
					MockerTmplTypes: mockerTmplTypes,
				})
			}

			typeParams := ""
			if len(typeParamArray) > 0 {
				typeParams = "[" + strings.Join(typeParamArray, ", ") + "]"
			}

			typeParamNames := ""
			if len(typeParamNameArray) > 0 {
				typeParamNames = "[" + strings.Join(typeParamNameArray, ", ") + "]"
			}

			ret = append(ret, Interface{
				Package:         node.Name.String(),
				Name:            name,
				TypeParams:      typeParams,
				TypeParamNames:  typeParamNames,
				EmbedInterfaces: embedInterfaces.String(),
				Methods:         methods,
				File:            file,
				Imports:         needImports,
			})
		}
	}
	return ret
}

var (
	typeTextBuffer  bytes.Buffer
	typeTextFileSet = token.NewFileSet()
	pkgNameSelector = regexp.MustCompile(`([a-zA-Z0-9_]+\.)`) // Matches package prefixes in type expressions
)

// getTypeText converts an AST type expression to its string representation,
// and returns a slice of package names used in the type.
func getTypeText(t ast.Expr) (typeText string, pkgNames []string) {
	typeTextBuffer.Reset()
	_ = printer.Fprint(&typeTextBuffer, typeTextFileSet, t)
	typeText = typeTextBuffer.String()
	pkgNames = pkgNameSelector.FindAllString(typeText, -1)
	return
}
