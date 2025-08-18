package golangci

import (
	"github.com/ovargas/wrap-error-linter/pkg/analyzer"
	"golang.org/x/tools/go/analysis"
)

func New(settings map[string]interface{}) *analysis.Analyzer {
	return analyzer.Analyzer
}

func GetAnalyzer() *analysis.Analyzer {
	return analyzer.Analyzer
}