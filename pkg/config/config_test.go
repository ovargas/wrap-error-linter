package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsPackageExcluded(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		pkg      string
		expected bool
	}{
		{
			name: "exact match",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{"github.com/example/mocks"},
				},
			},
			pkg:      "github.com/example/mocks",
			expected: true,
		},
		{
			name: "no match",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{"github.com/example/mocks"},
				},
			},
			pkg:      "github.com/example/service",
			expected: false,
		},
		{
			name: "double star pattern - mocks at end",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{"**/mocks"},
				},
			},
			pkg:      "github.com/example/pkg/mocks",
			expected: true,
		},
		{
			name: "double star pattern - mocks in middle with subpackages",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{"**/mocks/*"},
				},
			},
			pkg:      "github.com/legalsifter/ms-playbook/internal/mocks/provision/persistence",
			expected: true,
		},
		{
			name: "double star pattern - mocks in middle",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{"**/mocks/*"},
				},
			},
			pkg:      "github.com/example/mocks/database",
			expected: true,
		},
		{
			name: "double star pattern - no match when mocks not in path",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{"**/mocks/*"},
				},
			},
			pkg:      "github.com/example/service/database",
			expected: false,
		},
		{
			name: "double star pattern - no match when mocks is at end without wildcard",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{"**/mocks/*"},
				},
			},
			pkg:      "github.com/example/mocks",
			expected: false,
		},
		{
			name: "standard glob pattern",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{"github.com/*/mocks"},
				},
			},
			pkg:      "github.com/example/mocks",
			expected: true,
		},
		{
			name: "standard glob pattern - no match",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{"github.com/*/mocks"},
				},
			},
			pkg:      "github.com/example/pkg/mocks",
			expected: false,
		},
		{
			name: "multiple patterns",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{
						"**/mocks",
						"**/generated",
						"github.com/specific/package",
					},
				},
			},
			pkg:      "github.com/example/generated",
			expected: true,
		},
		{
			name: "complex real-world case",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{
						"**/mocks/*",
						"**/testdata",
						"**/vendor/*",
					},
				},
			},
			pkg:      "github.com/legalsifter/ms-playbook/internal/mocks/provision/persistence",
			expected: true,
		},
		{
			name: "empty exclude list",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: []string{},
				},
			},
			pkg:      "github.com/example/mocks",
			expected: false,
		},
		{
			name: "nil exclude packages",
			config: &Config{
				Exclude: ExcludeConfig{
					Packages: nil,
				},
			},
			pkg:      "github.com/example/mocks",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsPackageExcluded(tt.pkg)
			if result != tt.expected {
				t.Errorf("IsPackageExcluded(%q) = %v, want %v", tt.pkg, result, tt.expected)
			}
		})
	}
}

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		filename string
		expected bool
	}{
		{
			name: "test file excluded",
			config: &Config{
				Exclude: ExcludeConfig{
					Files: []string{"*_test.go"},
				},
			},
			filename: "service_test.go",
			expected: true,
		},
		{
			name: "generated file excluded",
			config: &Config{
				Exclude: ExcludeConfig{
					Files: []string{"generated_*.go"},
				},
			},
			filename: "generated_client.go",
			expected: true,
		},
		{
			name: "protobuf file excluded",
			config: &Config{
				Exclude: ExcludeConfig{
					Files: []string{"*.pb.go"},
				},
			},
			filename: "service.pb.go",
			expected: true,
		},
		{
			name: "regular file not excluded",
			config: &Config{
				Exclude: ExcludeConfig{
					Files: []string{"*_test.go", "*.pb.go"},
				},
			},
			filename: "service.go",
			expected: false,
		},
		{
			name: "full path with pattern",
			config: &Config{
				Exclude: ExcludeConfig{
					Files: []string{"*_test.go"},
				},
			},
			filename: "/path/to/project/pkg/service_test.go",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsExcluded(tt.filename)
			if result != tt.expected {
				t.Errorf("IsExcluded(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestIsTrustedPackage(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		pkg      string
		expected bool
	}{
		{
			name: "trusted package",
			config: &Config{
				TrustedPackages: []string{
					"github.com/mycompany/internal",
					"github.com/mycompany/common",
				},
			},
			pkg:      "github.com/mycompany/internal",
			expected: true,
		},
		{
			name: "not trusted package",
			config: &Config{
				TrustedPackages: []string{
					"github.com/mycompany/internal",
				},
			},
			pkg:      "github.com/external/package",
			expected: false,
		},
		{
			name: "empty trusted list",
			config: &Config{
				TrustedPackages: []string{},
			},
			pkg:      "github.com/mycompany/internal",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsTrustedPackage(tt.pkg)
			if result != tt.expected {
				t.Errorf("IsTrustedPackage(%q) = %v, want %v", tt.pkg, result, tt.expected)
			}
		})
	}
}

func TestGetSeverity(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		rule     string
		expected Severity
	}{
		{
			name: "configured severity",
			config: &Config{
				Severity: map[string]Severity{
					"unwrapped-external-error": SeverityError,
					"double-wrap":              SeverityWarn,
				},
			},
			rule:     "unwrapped-external-error",
			expected: SeverityError,
		},
		{
			name: "default severity",
			config: &Config{
				Severity: map[string]Severity{
					"unwrapped-external-error": SeverityError,
				},
			},
			rule:     "unknown-rule",
			expected: SeverityWarn,
		},
		{
			name:     "nil severity map",
			config:   &Config{},
			rule:     "any-rule",
			expected: SeverityWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetSeverity(tt.rule)
			if result != tt.expected {
				t.Errorf("GetSeverity(%q) = %v, want %v", tt.rule, result, tt.expected)
			}
		})
	}
}

func TestShouldFail(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name:     "fail mode",
			config:   &Config{Mode: "fail"},
			expected: true,
		},
		{
			name:     "warn mode",
			config:   &Config{Mode: "warn"},
			expected: false,
		},
		{
			name:     "empty mode defaults to warn",
			config:   &Config{Mode: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ShouldFail()
			if result != tt.expected {
				t.Errorf("ShouldFail() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".wrap-error-linter.yml")

	yamlContent := `
mode: fail
output: json
max-wrap-depth: 5
require-context: true
exclude:
  packages:
    - "**/mocks/*"
    - "**/generated"
  files:
    - "*_test.go"
severity:
  unwrapped-external-error: error
  double-wrap: warn
trusted-packages:
  - "github.com/mycompany/internal"
`

	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	tests := []struct {
		name           string
		configPath     string
		expectedMode   string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "load existing config",
			configPath:     configPath,
			expectedMode:   "fail",
			expectedOutput: "json",
			expectError:    false,
		},
		{
			name:           "load non-existent config returns default",
			configPath:     "/non/existent/path/config.yml",
			expectedMode:   "warn",
			expectedOutput: "text",
			expectError:    true,
		},
		{
			name:           "empty path uses default",
			configPath:     "",
			expectedMode:   "warn",
			expectedOutput: "text",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(tt.configPath)
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if cfg != nil {
				if cfg.Mode != tt.expectedMode {
					t.Errorf("Mode = %q, want %q", cfg.Mode, tt.expectedMode)
				}
				if cfg.Output != tt.expectedOutput {
					t.Errorf("Output = %q, want %q", cfg.Output, tt.expectedOutput)
				}
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig

	// Test default values
	if cfg.Mode != "warn" {
		t.Errorf("Default Mode = %q, want %q", cfg.Mode, "warn")
	}
	if cfg.Output != "text" {
		t.Errorf("Default Output = %q, want %q", cfg.Output, "text")
	}
	if cfg.MaxWrapDepth != 10 {
		t.Errorf("Default MaxWrapDepth = %d, want %d", cfg.MaxWrapDepth, 10)
	}
	if cfg.RequireContext != false {
		t.Errorf("Default RequireContext = %v, want %v", cfg.RequireContext, false)
	}
	if !cfg.IgnoreSentinelErrors {
		t.Errorf("Default IgnoreSentinelErrors = %v, want %v", cfg.IgnoreSentinelErrors, true)
	}

	// Test default custom wrappers
	if !cfg.CustomWrappers.AutoDetectUnwrap {
		t.Error("Default AutoDetectUnwrap should be true")
	}
	if len(cfg.CustomWrappers.Packages) == 0 {
		t.Error("Default CustomWrappers should include common packages")
	}

	// Test default severity levels
	if cfg.GetSeverity("unwrapped-external-error") != SeverityWarn {
		t.Error("Default severity should be warn")
	}

	// Test default exclusions
	if len(cfg.Exclude.Files) == 0 {
		t.Error("Default exclusions should include test files")
	}
}

func TestFindConfigFile(t *testing.T) {
	// This test is environment-dependent, so we'll just ensure it doesn't panic
	result := findConfigFile()
	// Result can be empty string if no config file is found
	_ = result
}

func TestConfigWithComplexPatterns(t *testing.T) {
	config := &Config{
		Exclude: ExcludeConfig{
			Packages: []string{
				"**/mocks/*",
				"**/testdata",
				"**/vendor/*",
				"github.com/specific/exact",
				"*.test",
			},
		},
	}

	testCases := []struct {
		pkg      string
		excluded bool
	}{
		// Test **/mocks/* pattern - the main test case requested by the user
		{"github.com/legalsifter/ms-playbook/internal/mocks/provision/persistence", true},
		{"github.com/example/mocks/database", true},
		{"github.com/example/pkg/mocks/client", true},
		{"github.com/example/mocks", false}, // No trailing component after mocks
		
		// Test **/testdata pattern
		{"github.com/example/testdata", true},
		{"github.com/example/pkg/testdata", true},
		{"github.com/example/testdata/subfolder", false}, // Pattern doesn't include /*
		
		// Test **/vendor/* pattern
		{"github.com/example/vendor/dependency", true},
		{"github.com/example/vendor/github.com/other/pkg", true},
		{"github.com/example/vendor", false}, // No trailing component
		
		// Test exact match
		{"github.com/specific/exact", true},
		{"github.com/specific/exact/sub", false},
		
		// Test *.test pattern
		{"mypackage.test", true},
		{"other.test", true},
		{"nottest", false},
		
		// Non-matching packages
		{"github.com/example/service", false},
		{"github.com/example/api/handlers", false},
	}

	for _, tc := range testCases {
		t.Run(tc.pkg, func(t *testing.T) {
			result := config.IsPackageExcluded(tc.pkg)
			if result != tc.excluded {
				t.Errorf("IsPackageExcluded(%q) = %v, want %v", tc.pkg, result, tc.excluded)
			}
		})
	}
}

// TestSpecificMocksPattern tests the exact case mentioned by the user
func TestSpecificMocksPattern(t *testing.T) {
	config := &Config{
		Exclude: ExcludeConfig{
			Packages: []string{"**/mocks/*"},
		},
	}

	// This is the exact test case the user requested
	pkg := "github.com/legalsifter/ms-playbook/internal/mocks/provision/persistence"
	result := config.IsPackageExcluded(pkg)
	if !result {
		t.Errorf("Expected package %q to be excluded by pattern '**/mocks/*', but it was not", pkg)
	}
}

func TestVariousMockPatterns(t *testing.T) {
	testCases := []struct {
		pattern  string
		pkg      string
		excluded bool
	}{
		// Different variations of mocks patterns
		{"**/mocks/*", "github.com/company/service/mocks/db", true},
		{"**/mocks/*", "github.com/company/service/mocks/api/client", true},
		{"**/mocks/*", "github.com/company/service/mocks", false}, // No subpath
		{"**/mocks", "github.com/company/service/mocks", true},    // Ends with mocks
		{"**/mocks", "github.com/company/service/mocks/db", false}, // Has subpath but pattern doesn't allow it
		{"mocks/*", "mocks/db", true},     // Simple case
		{"mocks/*", "service/mocks/db", false}, // Doesn't match prefix
	}

	for _, tc := range testCases {
		config := &Config{
			Exclude: ExcludeConfig{
				Packages: []string{tc.pattern},
			},
		}

		t.Run(tc.pattern+"_"+tc.pkg, func(t *testing.T) {
			result := config.IsPackageExcluded(tc.pkg)
			if result != tc.excluded {
				t.Errorf("Pattern %q with package %q: got %v, want %v", tc.pattern, tc.pkg, result, tc.excluded)
			}
		})
	}
}