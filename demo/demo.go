package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"strings"

	"github.com/ovargas/wrap-error-linter/pkg/analyzer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

func main() {
	src := `
package example

import (
	"fmt"
	"os"
)

func badExample() error {
	file, err := os.Open("test.txt")
	if err != nil {
		return err // Should trigger: error from external package should be wrapped
	}
	defer file.Close()
	return nil
}

func goodExample() error {
	file, err := os.Open("test.txt")
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err) // OK - properly wrapped
	}
	defer file.Close()
	return nil
}

func wrongVerbExample() error {
	file, err := os.Open("test.txt")
	if err != nil {
		return fmt.Errorf("failed: %v", err) // Should trigger: use %w instead of %v
	}
	defer file.Close()
	return nil
}
`

	// Parse the source code
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, parser.AllErrors)
	if err != nil {
		log.Fatal(err)
	}

	// Type-check the package
	conf := types.Config{
		Importer: importer.Default(),
	}
	
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	pkg, err := conf.Check("example", fset, []*ast.File{file}, info)
	if err != nil {
		// Type errors are expected for this demo
		fmt.Printf("Type checking completed with errors (expected): %v\n\n", err)
	}

	// Create inspector
	insp := inspector.New([]*ast.File{file})

	// Create the pass
	pass := &analysis.Pass{
		Analyzer:  analyzer.Analyzer,
		Fset:      fset,
		Files:     []*ast.File{file},
		Pkg:       pkg,
		TypesInfo: info,
		ResultOf: map[*analysis.Analyzer]interface{}{
			inspect.Analyzer: insp,
		},
		Report: func(d analysis.Diagnostic) {
			pos := fset.Position(d.Pos)
			fmt.Printf("Line %d: %s\n", pos.Line, d.Message)
		},
	}

	// Run the analyzer
	fmt.Println("Running wrap-error-linter analysis:")
	fmt.Println("=" + strings.Repeat("=", 50))
	
	result, err := analyzer.Analyzer.Run(pass)
	if err != nil {
		log.Printf("Error: %v\n", err)
	}
	
	if issues, ok := result.([]analyzer.Issue); ok {
		if len(issues) == 0 {
			fmt.Println("No issues found!")
		} else {
			for _, issue := range issues {
				pos := fset.Position(issue.Diagnostic.Pos)
				fmt.Printf("Line %d: [%s] %s\n", pos.Line, issue.Rule, issue.Diagnostic.Message)
			}
		}
	}
}