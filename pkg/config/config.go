package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

type Config struct {
	Mode                 string              `yaml:"mode"`
	Output               string              `yaml:"output"`
	OutputPath           string              `yaml:"output-path"`
	IgnorePackages       []string            `yaml:"ignore-packages"`
	IgnoreSentinelErrors bool                `yaml:"ignore-sentinel-errors"`
	MaxWrapDepth         int                 `yaml:"max-wrap-depth"`
	RequireContext       bool                `yaml:"require-context"`
	ContextMinLength     int                 `yaml:"context-min-length"`
	ContextPatterns      []string            `yaml:"context-patterns"`
	CustomWrappers       CustomWrappers      `yaml:"custom-wrappers"`
	Severity             map[string]Severity `yaml:"severity"`
	Exclude              ExcludeConfig       `yaml:"exclude"`
	TrustedPackages      []string            `yaml:"trusted-packages"`
}

type CustomWrappers struct {
	Packages         []PackageWrapper `yaml:"packages"`
	AutoDetectUnwrap bool             `yaml:"auto-detect-unwrap"`
}

type PackageWrapper struct {
	Package   string   `yaml:"package"`
	Functions []string `yaml:"functions"`
}

type ExcludeConfig struct {
	Files           []string `yaml:"files"`
	Functions       []string `yaml:"functions"`
	Packages        []string `yaml:"packages"`
	AllowDirectives bool     `yaml:"allow-directives"`
}

var DefaultConfig = Config{
	Mode:                 "warn",
	Output:               "text",
	OutputPath:           "",
	IgnoreSentinelErrors: true,
	MaxWrapDepth:         10,
	RequireContext:       false,
	ContextMinLength:     10,
	CustomWrappers: CustomWrappers{
		AutoDetectUnwrap: true,
		Packages: []PackageWrapper{
			{
				Package:   "github.com/pkg/errors",
				Functions: []string{"Wrap", "Wrapf", "WithStack", "WithMessage"},
			},
			{
				Package:   "github.com/go-errors/errors",
				Functions: []string{"Wrap", "WrapPrefix"},
			},
			{
				Package:   "golang.org/x/xerrors",
				Functions: []string{"Errorf"},
			},
			{
				Package:   "github.com/hashicorp/go-multierror",
				Functions: []string{"Append"},
			},
			{
				Package:   "go.uber.org/multierr",
				Functions: []string{"Append", "Combine"},
			},
		},
	},
	Severity: map[string]Severity{
		"unwrapped-external-error": SeverityWarn,
		"double-wrap":              SeverityWarn,
		"missing-context":          SeverityWarn,
		"using-percent-v":          SeverityWarn,
		"max-depth-exceeded":       SeverityWarn,
	},
	Exclude: ExcludeConfig{
		Files:           []string{"*_test.go", "generated_*.go", "*.pb.go"},
		AllowDirectives: true,
	},
	IgnorePackages: []string{
		"io",
		"database/sql",
	},
}

func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig

	if path == "" {
		path = findConfigFile()
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	if config.MaxWrapDepth == 0 {
		config.MaxWrapDepth = 10
	}

	return &config, nil
}

func findConfigFile() string {
	possibleNames := []string{
		".wrap-error-linter.yml",
		"wrap-error-linter.yml",
		".wrap-error-linter.yaml",
		"wrap-error-linter.yaml",
	}

	dir, _ := os.Getwd()
	for dir != "/" && dir != "" {
		for _, name := range possibleNames {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		dir = filepath.Dir(dir)
	}

	homeDir, _ := os.UserHomeDir()
	for _, name := range possibleNames {
		path := filepath.Join(homeDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func (c *Config) IsExcluded(filename string) bool {
	for _, pattern := range c.Exclude.Files {
		matched, _ := filepath.Match(pattern, filepath.Base(filename))
		if matched {
			return true
		}
	}
	return false
}

func (c *Config) IsPackageExcluded(pkg string) bool {
	for _, pattern := range c.Exclude.Packages {
		// Support exact match
		if pkg == pattern {
			return true
		}

		// Support glob patterns with ** (matches any number of directories)
		if strings.Contains(pattern, "**") {
			// Handle patterns like "**/mocks/*" or "**/testdata"
			if strings.HasPrefix(pattern, "**/") {
				suffix := pattern[3:] // Remove the "**/" prefix

				// If pattern is "**/mocks/*", check if pkg contains "/mocks/"
				if strings.HasSuffix(suffix, "/*") {
					component := suffix[:len(suffix)-2] // Remove "/*"
					if strings.Contains(pkg, "/"+component+"/") {
						return true
					}
				} else {
					// If pattern is "**/mocks", only match if pkg ends with "/mocks"
					if strings.HasSuffix(pkg, "/"+suffix) {
						return true
					}
				}
			}
		}

		// Support standard glob patterns
		if strings.Contains(pattern, "*") {
			matched, _ := filepath.Match(pattern, pkg)
			if matched {
				return true
			}

			// Also try matching against the last part of the package path
			parts := strings.Split(pkg, "/")
			if len(parts) > 0 {
				matched, _ = filepath.Match(pattern, parts[len(parts)-1])
				if matched {
					return true
				}
			}
		}
	}
	return false
}

func (c *Config) IsTrustedPackage(pkg string) bool {
	for _, trusted := range c.TrustedPackages {
		if pkg == trusted {
			return true
		}
	}
	return false
}

func (c *Config) GetSeverity(rule string) Severity {
	if sev, ok := c.Severity[rule]; ok {
		return sev
	}
	return SeverityWarn
}

func (c *Config) ShouldFail() bool {
	return c.Mode == "fail"
}
