package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ovargas/wrap-error-linter/pkg/analyzer"
	"github.com/ovargas/wrap-error-linter/pkg/config"
	"github.com/ovargas/wrap-error-linter/pkg/reporter"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/packages"
)

var (
	configPath      = flag.String("config", "", "Path to configuration file")
	mode           = flag.String("mode", "", "Mode: warn or fail")
	output         = flag.String("output", "", "Output format: text, json, xml, html")
	outputPath     = flag.String("output-path", "", "Path to output file")
	requireContext = flag.Bool("require-context", false, "Require context message when wrapping errors")
	showVersion    = flag.Bool("version", false, "Show version information")
)

const version = "1.0.0"

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("wrap-error-linter version %s\n", version)
		os.Exit(0)
	}

	// Always use custom mode for better package loading
	runCustomMode()
}

func hasConfigOverrides() bool {
	return *configPath != "" || *mode != "" || *output != "" || *outputPath != "" || *requireContext
}

func runCustomMode() {
	cfg, err := loadConfiguration()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	applyCommandLineOverrides(cfg)

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	// Load packages with proper configuration
	pkgConfig := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedSyntax | packages.NeedModule | packages.NeedDeps,
		Tests: false,
		BuildFlags: []string{"-mod=readonly"},
	}

	pkgs, err := packages.Load(pkgConfig, patterns...)
	if err != nil {
		log.Fatalf("Failed to load packages: %v", err)
	}

	var allIssues []analyzer.Issue
	hasErrors := false

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			// Skip packages with compilation errors unless they're test files
			log.Printf("Skipping package %s due to compilation errors", pkg.PkgPath)
			continue
		}

		if pkg.Types == nil || pkg.TypesInfo == nil {
			log.Printf("Skipping package %s: missing type information", pkg.PkgPath)
			continue
		}

		// Skip excluded packages
		if cfg.IsPackageExcluded(pkg.PkgPath) {
			continue
		}

		issues, err := analyzePackage(pkg, cfg)
		if err != nil {
			log.Printf("Error analyzing package %s: %v", pkg.PkgPath, err)
			continue
		}

		allIssues = append(allIssues, issues...)

		// Check if we have errors that should fail the build
		for _, issue := range issues {
			if issue.Severity == config.SeverityError || 
			   (issue.Severity == config.SeverityWarn && cfg.ShouldFail()) {
				hasErrors = true
			}
		}
	}

	// Report issues
	rep := reporter.NewReporter(cfg)
	if len(allIssues) > 0 {
		// Create a dummy pass for reporting
		dummyPass := &analysis.Pass{
			Fset: pkgs[0].Fset,
		}
		if err := rep.Report(allIssues, dummyPass); err != nil {
			log.Fatalf("Failed to generate report: %v", err)
		}
	}

	if hasErrors && cfg.ShouldFail() {
		os.Exit(1)
	}
}

func analyzePackage(pkg *packages.Package, cfg *config.Config) ([]analyzer.Issue, error) {
	// Create analysis pass
	pass := &analysis.Pass{
		Analyzer:  analyzer.Analyzer,
		Fset:      pkg.Fset,
		Files:     pkg.Syntax,
		Pkg:       pkg.Types,
		TypesInfo: pkg.TypesInfo,
		Report:    func(d analysis.Diagnostic) {}, // We collect issues differently
		ResultOf:  make(map[*analysis.Analyzer]interface{}),
	}

	// Run inspect analyzer first
	inspectResult, err := inspect.Analyzer.Run(&analysis.Pass{
		Analyzer:  inspect.Analyzer,
		Fset:      pkg.Fset,
		Files:     pkg.Syntax,
		Pkg:       pkg.Types,
		TypesInfo: pkg.TypesInfo,
		Report:    func(d analysis.Diagnostic) {},
	})
	if err != nil {
		return nil, fmt.Errorf("inspect analyzer failed: %w", err)
	}

	pass.ResultOf[inspect.Analyzer] = inspectResult

	// Run our analyzer
	result, err := analyzer.Analyzer.Run(pass)
	if err != nil {
		return nil, fmt.Errorf("analyzer failed: %w", err)
	}

	if issues, ok := result.([]analyzer.Issue); ok {
		return issues, nil
	}

	return nil, nil
}

func loadConfiguration() (*config.Config, error) {
	if *configPath != "" {
		return config.LoadConfig(*configPath)
	}
	
	cfg, err := config.LoadConfig("")
	if err != nil {
		// Use default config if no config file found
		cfg = &config.DefaultConfig
	}
	return cfg, nil
}

func applyCommandLineOverrides(cfg *config.Config) {
	if *mode != "" {
		cfg.Mode = *mode
	}
	if *output != "" {
		cfg.Output = *output
	}
	if *outputPath != "" {
		cfg.OutputPath = *outputPath
	}
	if *requireContext {
		cfg.RequireContext = true
	}
}