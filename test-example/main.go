package main

import (
	"fmt"
	"log"

	"github.com/ovargas/wrap-error-linter/pkg/analyzer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

func main() {
	// Load the package
	cfg := &packages.Config{
		Mode: packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}

	pkgs, err := packages.Load(cfg, "./example")
	if err != nil {
		log.Fatal(err)
	}

	// Run the analyzer
	for _, pkg := range pkgs {
		// First run the inspect analyzer (required dependency)
		inspectPass := &analysis.Pass{
			Analyzer: analyzer.Analyzer.Requires[0],
			Fset:     pkg.Fset,
			Files:    pkg.Syntax,
			Pkg:      pkg.Types,
			TypesInfo: pkg.TypesInfo,
			Report: func(d analysis.Diagnostic) {},
		}
		
		inspectResult, err := analyzer.Analyzer.Requires[0].Run(inspectPass)
		if err != nil {
			log.Fatal(err)
		}

		// Now run our analyzer
		pass := &analysis.Pass{
			Analyzer: analyzer.Analyzer,
			Fset:     pkg.Fset,
			Files:    pkg.Syntax,
			Pkg:      pkg.Types,
			TypesInfo: pkg.TypesInfo,
			ResultOf: map[*analysis.Analyzer]interface{}{
				analyzer.Analyzer.Requires[0]: inspectResult,
			},
			Report: func(d analysis.Diagnostic) {
				fmt.Printf("%s: %s\n", pkg.Fset.Position(d.Pos), d.Message)
			},
		}

		result, err := analyzer.Analyzer.Run(pass)
		if err != nil {
			log.Printf("Error running analyzer: %v\n", err)
		} else {
			log.Printf("Analysis complete. Result type: %T\n", result)
		}
	}
}