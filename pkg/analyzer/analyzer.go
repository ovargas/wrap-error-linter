package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/ovargas/wrap-error-linter/pkg/config"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "wraperror",
	Doc:      "Checks that errors from external packages are properly wrapped",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

type Issue struct {
	Diagnostic analysis.Diagnostic
	Rule       string
	Severity   config.Severity
}

type analyzer struct {
	pass           *analysis.Pass
	config         *config.Config
	inspector      *inspector.Inspector
	errorType      types.Type
	wrappedErrors  map[types.Object]bool
	errorSources   map[types.Object]string  // tracks which package an error came from
	issues         []Issue
	currentPkgPath string
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Skip analysis if package is not complete
	if pass.Pkg == nil || pass.TypesInfo == nil {
		return nil, nil
	}

	cfg := &config.DefaultConfig
	
	// Try to load config, but don't fail if we can't
	if userCfg, err := config.LoadConfig(""); err == nil {
		cfg = userCfg
	}

	if len(pass.Files) > 0 {
		filename := pass.Fset.File(pass.Files[0].Pos()).Name()
		if cfg.IsExcluded(filename) {
			return nil, nil
		}
	}

	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		// Fallback: create our own inspector if not available
		inspect = inspector.New(pass.Files)
	}
	
	errorType := types.Universe.Lookup("error").Type()
	if errorType == nil {
		// Skip if we can't find the error type
		return nil, nil
	}

	a := &analyzer{
		pass:           pass,
		config:         cfg,
		inspector:      inspect,
		errorType:      errorType,
		wrappedErrors:  make(map[types.Object]bool),
		errorSources:   make(map[types.Object]string),
		currentPkgPath: pass.Pkg.Path(),
	}

	a.analyze()

	return a.issues, nil
}

func (a *analyzer) analyze() {
	nodeFilter := []ast.Node{
		(*ast.ReturnStmt)(nil),
		(*ast.CallExpr)(nil),
		(*ast.AssignStmt)(nil),
	}

	a.inspector.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.ReturnStmt:
			a.analyzeReturnStmt(node)
		case *ast.CallExpr:
			a.analyzeCallExpr(node)
		case *ast.AssignStmt:
			a.analyzeAssignStmt(node)
		}
	})
}

func (a *analyzer) analyzeReturnStmt(ret *ast.ReturnStmt) {
	for _, result := range ret.Results {
		if !a.isErrorType(result) {
			continue
		}

		if ident, ok := result.(*ast.Ident); ok {
			if ident.Name == "nil" {
				continue
			}

			obj := a.pass.TypesInfo.ObjectOf(ident)
			if obj == nil {
				continue
			}

			if !a.wrappedErrors[obj] {
				// Check if we know the source package of this error
				var pkg string
				if sourcePkg, ok := a.errorSources[obj]; ok {
					pkg = sourcePkg
				} else {
					pkg = a.getErrorPackage(obj)
				}
				
				if pkg != "" && pkg != a.currentPkgPath {
					if !a.config.IsTrustedPackage(pkg) && !a.isSentinelError(obj) {
						a.reportIssue(result.Pos(), "unwrapped-external-error", 
							"error from external package '%s' should be wrapped", pkg)
					}
				}
			} else {
				a.checkDoubleWrapping(result)
			}
		}

		if call, ok := result.(*ast.CallExpr); ok {
			a.analyzeErrorReturn(call)
		}
	}
}

func (a *analyzer) analyzeCallExpr(call *ast.CallExpr) {
	if a.isErrorWrappingCall(call) {
		if len(call.Args) > 0 {
			for _, arg := range call.Args {
				if a.isErrorType(arg) {
					if ident, ok := arg.(*ast.Ident); ok {
						obj := a.pass.TypesInfo.ObjectOf(ident)
						if obj != nil {
							if a.wrappedErrors[obj] {
								a.reportIssue(call.Pos(), "double-wrap",
									"error is already wrapped")
							}
							a.wrappedErrors[obj] = true
						}
					}
				}
			}
		}
		
		a.checkWrappingContext(call)
		a.checkPercentVUsage(call)
	}
}

func (a *analyzer) analyzeAssignStmt(assign *ast.AssignStmt) {
	// Handle multiple assignment (e.g., file, err := os.Open())
	if len(assign.Rhs) == 1 {
		if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
			// Check each LHS variable to see if it's an error
			for _, lhs := range assign.Lhs {
				if !a.isErrorType(lhs) {
					continue
				}

				if ident, ok := lhs.(*ast.Ident); ok {
					obj := a.pass.TypesInfo.ObjectOf(ident)
					if obj == nil {
						continue
					}

					if a.isErrorWrappingCall(call) {
						a.wrappedErrors[obj] = true
					} else {
						// Track the source package of this error
						if pkg := a.getCallPackage(call); pkg != "" && pkg != a.currentPkgPath {
							a.errorSources[obj] = pkg
						}
					}
				}
			}
		}
	} else {
		// Handle regular assignment (err = someCall())
		for i, lhs := range assign.Lhs {
			if i >= len(assign.Rhs) {
				continue
			}
			
			if !a.isErrorType(lhs) {
				continue
			}

			if ident, ok := lhs.(*ast.Ident); ok {
				obj := a.pass.TypesInfo.ObjectOf(ident)
				if obj == nil {
					continue
				}

				if call, ok := assign.Rhs[i].(*ast.CallExpr); ok {
					if a.isErrorWrappingCall(call) {
						a.wrappedErrors[obj] = true
					} else {
						// Track the source package of this error
						if pkg := a.getCallPackage(call); pkg != "" && pkg != a.currentPkgPath {
							a.errorSources[obj] = pkg
						}
					}
				}
			}
		}
	}
}

func (a *analyzer) isErrorType(expr ast.Expr) bool {
	typ := a.pass.TypesInfo.TypeOf(expr)
	if typ == nil {
		return false
	}
	return types.Implements(typ, a.errorType.Underlying().(*types.Interface))
}

func (a *analyzer) isErrorWrappingCall(call *ast.CallExpr) bool {
	if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
		if pkg, ok := fun.X.(*ast.Ident); ok {
			pkgName := pkg.Name
			funcName := fun.Sel.Name

			if pkgName == "fmt" && funcName == "Errorf" {
				return a.hasPercentW(call)
			}

			if pkgName == "errors" && (funcName == "Join" || funcName == "New") {
				return true
			}

			for _, wrapper := range a.config.CustomWrappers.Packages {
				if strings.HasSuffix(wrapper.Package, "/"+pkgName) || strings.HasSuffix(wrapper.Package, pkgName) {
					for _, fn := range wrapper.Functions {
						if fn == funcName {
							return true
						}
					}
				}
			}
		}
	}

	if a.config.CustomWrappers.AutoDetectUnwrap {
		if typ := a.pass.TypesInfo.TypeOf(call); typ != nil {
			if hasUnwrapMethod(typ) {
				return true
			}
		}
	}

	return false
}

func (a *analyzer) hasPercentW(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	
	if lit, ok := call.Args[0].(*ast.BasicLit); ok {
		return strings.Contains(lit.Value, "%w")
	}
	return false
}

func (a *analyzer) checkPercentVUsage(call *ast.CallExpr) {
	if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
		if pkg, ok := fun.X.(*ast.Ident); ok {
			if pkg.Name == "fmt" && fun.Sel.Name == "Errorf" {
				if len(call.Args) > 0 {
					if lit, ok := call.Args[0].(*ast.BasicLit); ok {
						if strings.Contains(lit.Value, "%v") && !strings.Contains(lit.Value, "%w") {
							for _, arg := range call.Args[1:] {
								if a.isErrorType(arg) {
									a.reportIssue(call.Pos(), "using-percent-v",
										"use %%w instead of %%v when wrapping errors")
									break
								}
							}
						}
					}
				}
			}
		}
	}
}

func (a *analyzer) checkWrappingContext(call *ast.CallExpr) {
	if !a.config.RequireContext {
		return
	}

	if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
		if pkg, ok := fun.X.(*ast.Ident); ok {
			if pkg.Name == "fmt" && fun.Sel.Name == "Errorf" && len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok {
					format := strings.Trim(lit.Value, `"`)
					
					if format == "%w" {
						a.reportIssue(call.Pos(), "missing-context",
							"error wrapping should include context message")
						return
					}

					contextPart := strings.Split(format, "%w")[0]
					if len(contextPart) < a.config.ContextMinLength {
						a.reportIssue(call.Pos(), "missing-context",
							"error context too short (minimum %d characters)", a.config.ContextMinLength)
					}

					if len(a.config.ContextPatterns) > 0 {
						matched := false
						for _, pattern := range a.config.ContextPatterns {
							if matchesPattern(format, pattern) {
								matched = true
								break
							}
						}
						if !matched {
							a.reportIssue(call.Pos(), "missing-context",
								"error context does not match required patterns")
						}
					}
				}
			}
		}
	}
}

func (a *analyzer) checkDoubleWrapping(expr ast.Expr) {
	
}

func (a *analyzer) analyzeErrorReturn(call *ast.CallExpr) {
	
}

func (a *analyzer) returnsErrorWithoutWrapping(call *ast.CallExpr) bool {
	return false
}

func (a *analyzer) getSourceError(call *ast.CallExpr) types.Object {
	return nil
}

func (a *analyzer) getErrorPackage(obj types.Object) string {
	if obj.Pkg() != nil {
		return obj.Pkg().Path()
	}
	return ""
}

func (a *analyzer) getCallPackage(call *ast.CallExpr) string {
	if call == nil {
		return ""
	}

	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		// Handle pkg.Function() calls
		if ident, ok := fn.X.(*ast.Ident); ok {
			if obj := a.pass.TypesInfo.ObjectOf(ident); obj != nil {
				if pkgName, ok := obj.(*types.PkgName); ok {
					return pkgName.Imported().Path()
				}
			}
		}
		
		// Handle obj.Method() calls - get the package of the method
		if obj := a.pass.TypesInfo.ObjectOf(fn.Sel); obj != nil {
			if obj.Pkg() != nil {
				return obj.Pkg().Path()
			}
		}
	case *ast.Ident:
		// Handle direct function calls
		if obj := a.pass.TypesInfo.ObjectOf(fn); obj != nil {
			if obj.Pkg() != nil {
				return obj.Pkg().Path()
			}
		}
	}
	
	return ""
}

func (a *analyzer) isSentinelError(obj types.Object) bool {
	if !a.config.IgnoreSentinelErrors {
		return false
	}

	name := obj.Name()
	pkg := a.getErrorPackage(obj)

	sentinelErrors := map[string][]string{
		"io":          {"EOF", "ErrClosedPipe", "ErrNoProgress", "ErrShortBuffer", "ErrShortWrite", "ErrUnexpectedEOF"},
		"database/sql": {"ErrNoRows", "ErrTxDone", "ErrConnDone"},
		"context":     {"Canceled", "DeadlineExceeded"},
	}

	if errors, ok := sentinelErrors[pkg]; ok {
		for _, sentinel := range errors {
			if name == sentinel {
				return true
			}
		}
	}

	return strings.HasPrefix(name, "Err")
}

func (a *analyzer) reportIssue(pos token.Pos, rule string, format string, args ...interface{}) {
	severity := a.config.GetSeverity(rule)
	
	diagnostic := analysis.Diagnostic{
		Pos:     pos,
		Message: fmt.Sprintf(format, args...),
	}
	
	issue := Issue{
		Diagnostic: diagnostic,
		Rule:       rule,
		Severity:   severity,
	}
	
	a.issues = append(a.issues, issue)
	
	if severity == config.SeverityError || (severity == config.SeverityWarn && a.config.ShouldFail()) {
		a.pass.Report(diagnostic)
	}
}

func hasUnwrapMethod(typ types.Type) bool {
	if typ == nil {
		return false
	}

	methods := types.NewMethodSet(typ)
	for i := 0; i < methods.Len(); i++ {
		method := methods.At(i).Obj()
		if method.Name() == "Unwrap" {
			sig, ok := method.Type().(*types.Signature)
			if ok && sig.Params().Len() == 0 && sig.Results().Len() == 1 {
				return true
			}
		}
	}
	return false
}

func matchesPattern(str, pattern string) bool {
	return false
}