package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Function represents a parsed function
type Function struct {
	Name        string
	Description string
	Params      []string
	Returns     []string
	Package     string
	File        string
}

// Package represents a parsed package
type Package struct {
	Name      string
	Functions []Function
}

func main() {
	fmt.Println("Sankarea Documentation Generator")
	fmt.Println("===============================")
	fmt.Println("Generated on 2025-05-22 14:50:06 by NullMeDev")
	fmt.Println()

	// Define directories to scan
	dirs := []string{
		"cmd/sankarea",
	}

	// Create output directory
	os.MkdirAll("docs", 0755)

	// Create overview markdown file
	overviewFile, err := os.Create("docs/README.md")
	if err != nil {
		fmt.Printf("Error creating overview file: %v\n", err)
		os.Exit(1)
	}
	defer overviewFile.Close()

	// Write header for overview
	writeOverviewHeader(overviewFile)

	// Generate package documentation
	packages := make(map[string]*Package)

	// Parse source files
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories and non-Go files
			if info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}

			// Parse the file
			functions, pkgName, err := parseFile(path)
			if err != nil {
				fmt.Printf("Error parsing %s: %v\n", path, err)
				return nil
			}

			// Add functions to the package
			if _, ok := packages[pkgName]; !ok {
				packages[pkgName] = &Package{Name: pkgName, Functions: []Function{}}
			}

			packages[pkgName].Functions = append(packages[pkgName].Functions, functions...)
			return nil
		})

		if err != nil {
			fmt.Printf("Error walking directory %s: %v\n", dir, err)
		}
	}

	// Write packages to overview
	for pkgName, pkg := range packages {
		fmt.Fprintf(overviewFile, "## Package: %s\n\n", pkgName)
		fmt.Fprintf(overviewFile, "Contains %d functions.\n\n", len(pkg.Functions))

		// Create package documentation file
		pkgFile, err := os.Create(fmt.Sprintf("docs/%s.md", pkgName))
		if err != nil {
			fmt.Printf("Error creating file for package %s: %v\n", pkgName, err)
			continue
		}

		// Write package documentation
		writePackageDoc(pkgFile, pkg)
		pkgFile.Close()

		// Add link to the overview
		fmt.Fprintf(overviewFile, "[View package documentation](%s.md)\n\n", pkgName)
	}

	// Create command-line documentation
	writeCommandDocs()

	fmt.Println("Documentation generation complete!")
	fmt.Println("Output written to the 'docs' directory.")
}

// parseFile parses a Go source file and extracts functions
func parseFile(filename string) ([]Function, string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, "", err
	}

	var functions []Function
	pkgName := node.Name.Name

	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			function := Function{
				Name:    fn.Name.Name,
				Package: pkgName,
				File:    filepath.Base(filename),
			}

			// Extract function description from comment
			if fn.Doc != nil {
				function.Description = fn.Doc.Text()
			}

			// Extract parameters
			if fn.Type.Params != nil {
				for _, param := range fn.Type.Params.List {
					paramType := astTypeToString(param.Type)
					for _, name := range param.Names {
						function.Params = append(function.Params, fmt.Sprintf("%s %s", name.Name, paramType))
					}
				}
			}

			// Extract return values
			if fn.Type.Results != nil {
				for _, result := range fn.Type.Results.List {
					resultType := astTypeToString(result.Type)
					if len(result.Names) > 0 {
						for _, name := range result.Names {
							function.Returns = append(function.Returns, fmt.Sprintf("%s %s", name.Name, resultType))
						}
					} else {
						function.Returns = append(function.Returns, resultType)
					}
				}
			}

			functions = append(functions, function)
		}
	}

	return functions, pkgName, nil
}

// astTypeToString converts an AST type to a string representation
func astTypeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return astTypeToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + astTypeToString(t.X)
	case *ast.ArrayType:
		return "[]" + astTypeToString(t.Elt)
	case *ast.MapType:
		return "map[" + astTypeToString(t.Key) + "]" + astTypeToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// writeOverviewHeader writes the header for the overview documentation
func writeOverviewHeader(file *os.File) {
	file.WriteString("# Sankarea Documentation\n\n")
	file.WriteString("Sankarea is a Discord bot that aggregates news from RSS feeds, performs fact-checking, and provides news digests to Discord channels.\n\n")
	file.WriteString(fmt.Sprintf("Generated on %s by %s\n\n", time.Now().Format("2006-01-02 15:04:05"), "NullMeDev"))
	file.WriteString("## Table of Contents\n\n")
	file.WriteString("- [Installation](#installation)\n")
	file.WriteString("- [Configuration](#configuration)\n")
	file.WriteString("- [Command-line Usage](#command-line-usage)\n")
	file.WriteString("- [Packages](#packages)\n\n")

	file.WriteString("## Installation\n\n")
	file.WriteString("To install Sankarea, follow these steps:\n\n")
	file.WriteString("1. Clone the repository\n")
	file.WriteString("2. Run `./install.sh` to set up the environment\n")
	file.WriteString("3. Configure your Discord bot token and other settings in `.env`\n")
	file.WriteString("4. Run `make build` to build the application\n")
	file.WriteString("5. Start the bot with `./bin/sankarea`\n\n")

	file.WriteString("## Configuration\n\n")
	file.WriteString("Sankarea uses the following configuration files:\n\n")
	file.WriteString("- `config/config.json`: Main configuration file\n")
	file.WriteString("- `config/sources.yml`: News sources configuration\n")
	file.WriteString("- `.env`: Environment variables for API keys and tokens\n\n")

	file.WriteString("### Environment Variables\n\n")
	file.WriteString
