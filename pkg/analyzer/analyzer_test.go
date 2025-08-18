package analyzer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ovargas/wrap-error-linter/pkg/analyzer"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	
	testdata := filepath.Join(wd, "..", "..", "testdata")
	
	tests := []struct {
		name string
		dir  string
	}{
		{
			name: "basic unwrapped errors",
			dir:  "basic",
		},
		{
			name: "double wrapping",
			dir:  "double_wrap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysistest.Run(t, filepath.Join(testdata, tt.dir), analyzer.Analyzer, ".")
		})
	}
}