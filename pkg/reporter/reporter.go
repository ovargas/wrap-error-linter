package reporter

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/ovargas/wrap-error-linter/pkg/analyzer"
	"github.com/ovargas/wrap-error-linter/pkg/config"
	"golang.org/x/tools/go/analysis"
)

type Reporter interface {
	Report(issues []analyzer.Issue, pass *analysis.Pass) error
}

type ReportIssue struct {
	File     string `json:"file" xml:"file"`
	Line     int    `json:"line" xml:"line"`
	Column   int    `json:"column" xml:"column"`
	Rule     string `json:"rule" xml:"rule"`
	Severity string `json:"severity" xml:"severity"`
	Message  string `json:"message" xml:"message"`
}

type Report struct {
	Issues  []ReportIssue `json:"issues" xml:"issues"`
	Summary struct {
		Total    int `json:"total" xml:"total"`
		Errors   int `json:"errors" xml:"errors"`
		Warnings int `json:"warnings" xml:"warnings"`
		Info     int `json:"info" xml:"info"`
	} `json:"summary" xml:"summary"`
}

func NewReporter(cfg *config.Config) Reporter {
	var writer io.Writer = os.Stdout

	if cfg.OutputPath != "" {
		file, err := os.Create(cfg.OutputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
			writer = os.Stdout
		} else {
			writer = file
		}
	}

	switch cfg.Output {
	case "json":
		return &JSONReporter{writer: writer}
	case "xml":
		return &XMLReporter{writer: writer}
	case "html":
		return &HTMLReporter{writer: writer}
	default:
		return &TextReporter{writer: writer}
	}
}

type TextReporter struct {
	writer io.Writer
}

func (r *TextReporter) Report(issues []analyzer.Issue, pass *analysis.Pass) error {
	if len(issues) == 0 {
		return nil
	}

	sortedIssues := make([]analyzer.Issue, len(issues))
	copy(sortedIssues, issues)
	sort.Slice(sortedIssues, func(i, j int) bool {
		return sortedIssues[i].Diagnostic.Pos < sortedIssues[j].Diagnostic.Pos
	})

	for _, issue := range sortedIssues {
		pos := pass.Fset.Position(issue.Diagnostic.Pos)
		severityLabel := getSeverityLabel(issue.Severity)

		fmt.Fprintf(r.writer, "%s:%d:%d: [%s][%s] %s\n",
			pos.Filename,
			pos.Line,
			pos.Column,
			severityLabel,
			issue.Rule,
			issue.Diagnostic.Message,
		)
	}

	return nil
}

type JSONReporter struct {
	writer io.Writer
}

func (r *JSONReporter) Report(issues []analyzer.Issue, pass *analysis.Pass) error {
	report := buildReport(issues, pass)

	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("failed to encode report to JSON: %w", err)
	}

	return nil
}

type XMLReporter struct {
	writer io.Writer
}

func (r *XMLReporter) Report(issues []analyzer.Issue, pass *analysis.Pass) error {
	report := buildReport(issues, pass)

	encoder := xml.NewEncoder(r.writer)
	encoder.Indent("", "  ")

	if _, err := r.writer.Write([]byte(xml.Header)); err != nil {
		return fmt.Errorf("unable to write file: %w", err)
	}

	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("failed to encode report to XML: %w", err)
	}

	return nil
}

type HTMLReporter struct {
	writer io.Writer
}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Error Wrapping Linter Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        .summary { background: #f0f0f0; padding: 10px; margin: 20px 0; border-radius: 5px; }
        .summary span { margin-right: 20px; }
        .error { color: #d32f2f; }
        .warning { color: #f57c00; }
        .info { color: #0288d1; }
        table { width: 100%; border-collapse: collapse; }
        th { background: #333; color: white; padding: 10px; text-align: left; }
        td { padding: 8px; border-bottom: 1px solid #ddd; }
        tr:hover { background: #f5f5f5; }
        .file-path { font-family: monospace; font-size: 12px; }
    </style>
</head>
<body>
    <h1>Error Wrapping Linter Report</h1>
    <div class="summary">
        <strong>Summary:</strong>
        <span>Total: {{.Summary.Total}}</span>
        <span class="error">Errors: {{.Summary.Errors}}</span>
        <span class="warning">Warnings: {{.Summary.Warnings}}</span>
        <span class="info">Info: {{.Summary.Info}}</span>
    </div>
    <table>
        <thead>
            <tr>
                <th>File</th>
                <th>Line</th>
                <th>Severity</th>
                <th>Rule</th>
                <th>Message</th>
            </tr>
        </thead>
        <tbody>
            {{range .Issues}}
            <tr>
                <td class="file-path">{{.File}}</td>
                <td>{{.Line}}:{{.Column}}</td>
                <td class="{{.Severity}}">{{.Severity}}</td>
                <td>{{.Rule}}</td>
                <td>{{.Message}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</body>
</html>`

func (r *HTMLReporter) Report(issues []analyzer.Issue, pass *analysis.Pass) error {
	report := buildReport(issues, pass)

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("unable parse html template: %w", err)
	}

	if err := tmpl.Execute(r.writer, report); err != nil {
		return fmt.Errorf("unable to execute html template: %w", err)
	}

	return nil
}

func buildReport(issues []analyzer.Issue, pass *analysis.Pass) *Report {
	report := &Report{
		Issues: make([]ReportIssue, 0, len(issues)),
	}

	for _, issue := range issues {
		pos := pass.Fset.Position(issue.Diagnostic.Pos)

		reportIssue := ReportIssue{
			File:     filepath.Clean(pos.Filename),
			Line:     pos.Line,
			Column:   pos.Column,
			Rule:     issue.Rule,
			Severity: string(issue.Severity),
			Message:  issue.Diagnostic.Message,
		}

		report.Issues = append(report.Issues, reportIssue)

		switch issue.Severity {
		case config.SeverityError:
			report.Summary.Errors++
		case config.SeverityWarn:
			report.Summary.Warnings++
		case config.SeverityInfo:
			report.Summary.Info++
		}
	}

	report.Summary.Total = len(issues)

	sort.Slice(report.Issues, func(i, j int) bool {
		if report.Issues[i].File != report.Issues[j].File {
			return report.Issues[i].File < report.Issues[j].File
		}
		return report.Issues[i].Line < report.Issues[j].Line
	})

	return report
}

func getSeverityLabel(severity config.Severity) string {
	switch severity {
	case config.SeverityError:
		return "ERROR"
	case config.SeverityWarn:
		return "WARN"
	case config.SeverityInfo:
		return "INFO"
	default:
		return "WARN"
	}
}
