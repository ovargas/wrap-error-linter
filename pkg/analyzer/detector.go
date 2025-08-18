package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/ovargas/wrap-error-linter/internal/astutil"
	"github.com/ovargas/wrap-error-linter/internal/pkgutil"
)

type ErrorInfo struct {
	Origin     ast.Expr
	Package    string
	IsWrapped  bool
	WrapDepth  int
	WrapLocation token.Pos
}

type Detector struct {
	analyzer *analyzer
	errors   map[ast.Expr]*ErrorInfo
	scopes   map[*ast.BlockStmt]map[string]*ErrorInfo
}

func NewDetector(a *analyzer) *Detector {
	return &Detector{
		analyzer: a,
		errors:   make(map[ast.Expr]*ErrorInfo),
		scopes:   make(map[*ast.BlockStmt]map[string]*ErrorInfo),
	}
}

func (d *Detector) AnalyzeError(expr ast.Expr) *ErrorInfo {
	if expr == nil {
		return nil
	}

	if info, exists := d.errors[expr]; exists {
		return info
	}

	info := &ErrorInfo{
		Origin: expr,
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if e.Name == "nil" {
			return nil
		}
		info = d.analyzeIdentError(e)
		
	case *ast.CallExpr:
		info = d.analyzeCallError(e)
		
	case *ast.SelectorExpr:
		info = d.analyzeSelectorError(e)
	}

	if info != nil {
		d.errors[expr] = info
	}

	return info
}

func (d *Detector) analyzeIdentError(ident *ast.Ident) *ErrorInfo {
	obj := d.analyzer.pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return nil
	}

	info := &ErrorInfo{
		Origin:  ident,
		Package: pkgutil.GetPackagePath(obj),
	}

	if varObj, ok := obj.(*types.Var); ok {
		if scopeInfo := d.findInScope(varObj.Name()); scopeInfo != nil {
			info.IsWrapped = scopeInfo.IsWrapped
			info.WrapDepth = scopeInfo.WrapDepth
			info.WrapLocation = scopeInfo.WrapLocation
		}
	}

	if d.analyzer.wrappedErrors[obj] {
		info.IsWrapped = true
	}

	return info
}

func (d *Detector) analyzeCallError(call *ast.CallExpr) *ErrorInfo {
	info := &ErrorInfo{
		Origin:  call,
		Package: pkgutil.GetCallPackage(call, d.analyzer.pass.TypesInfo),
	}

	if d.isWrappingCall(call) {
		info.IsWrapped = true
		info.WrapLocation = call.Pos()
		
		if sourceErr := d.getWrappedError(call); sourceErr != nil {
			if sourceInfo := d.AnalyzeError(sourceErr); sourceInfo != nil {
				info.WrapDepth = sourceInfo.WrapDepth + 1
				if sourceInfo.IsWrapped {
					d.analyzer.reportIssue(call.Pos(), "double-wrap",
						"error is already wrapped at depth %d", sourceInfo.WrapDepth)
				}
			}
		}
	} else {
		if sourceErr := d.getReturnedError(call); sourceErr != nil {
			if sourceInfo := d.AnalyzeError(sourceErr); sourceInfo != nil {
				info.IsWrapped = sourceInfo.IsWrapped
				info.WrapDepth = sourceInfo.WrapDepth
			}
		}
	}

	return info
}

func (d *Detector) analyzeSelectorError(sel *ast.SelectorExpr) *ErrorInfo {
	obj := d.analyzer.pass.TypesInfo.ObjectOf(sel.Sel)
	if obj == nil {
		return &ErrorInfo{Origin: sel}
	}

	info := &ErrorInfo{
		Origin:  sel,
		Package: pkgutil.GetPackagePath(obj),
	}

	if d.isSentinelError(obj) {
		info.IsWrapped = false
	}

	return info
}

func (d *Detector) isWrappingCall(call *ast.CallExpr) bool {
	if d.analyzer.isErrorWrappingCall(call) {
		return true
	}

	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		funcName := sel.Sel.Name
		
		if pkg, ok := sel.X.(*ast.Ident); ok {
			pkgName := pkg.Name
			
			if pkgName == "fmt" && funcName == "Errorf" {
				if format, ok := astutil.FindFormatString(call); ok {
					return astutil.HasPercentW(format)
				}
			}
			
			if pkgName == "errors" {
				switch funcName {
				case "Join", "New":
					return true
				}
			}
		}
	}

	typ := d.analyzer.pass.TypesInfo.TypeOf(call)
	return pkgutil.HasUnwrapMethod(typ)
}

func (d *Detector) getWrappedError(call *ast.CallExpr) ast.Expr {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok {
			if pkg.Name == "fmt" && sel.Sel.Name == "Errorf" {
				for i := 1; i < len(call.Args); i++ {
					if pkgutil.IsErrorType(d.analyzer.pass.TypesInfo.TypeOf(call.Args[i])) {
						return call.Args[i]
					}
				}
			}
			
			if pkg.Name == "errors" && sel.Sel.Name == "Join" {
				if len(call.Args) > 0 {
					return call.Args[0]
				}
			}
		}
	}

	for _, arg := range call.Args {
		if pkgutil.IsErrorType(d.analyzer.pass.TypesInfo.TypeOf(arg)) {
			return arg
		}
	}

	return nil
}

func (d *Detector) getReturnedError(call *ast.CallExpr) ast.Expr {
	sig := astutil.GetFunctionSignature(call.Fun, d.analyzer.pass.TypesInfo)
	if sig == nil || sig.Results() == nil {
		return nil
	}

	for i := 0; i < sig.Results().Len(); i++ {
		if pkgutil.IsErrorType(sig.Results().At(i).Type()) {
			return call
		}
	}

	return nil
}

func (d *Detector) isSentinelError(obj types.Object) bool {
	if !d.analyzer.config.IgnoreSentinelErrors {
		return false
	}

	name := obj.Name()
	pkg := pkgutil.GetPackagePath(obj)

	sentinelErrors := map[string][]string{
		"io":           {"EOF", "ErrClosedPipe", "ErrNoProgress", "ErrShortBuffer", "ErrShortWrite", "ErrUnexpectedEOF"},
		"database/sql": {"ErrNoRows", "ErrTxDone", "ErrConnDone"},
		"context":      {"Canceled", "DeadlineExceeded"},
		"os":          {"ErrInvalid", "ErrPermission", "ErrExist", "ErrNotExist", "ErrClosed"},
		"net":         {"ErrClosed"},
	}

	if errors, ok := sentinelErrors[pkg]; ok {
		for _, sentinel := range errors {
			if name == sentinel {
				return true
			}
		}
	}

	if strings.HasPrefix(name, "Err") && obj.Exported() {
		if _, ok := obj.(*types.Var); ok {
			return true
		}
	}

	return false
}

func (d *Detector) TrackAssignment(lhs, rhs ast.Expr, scope *ast.BlockStmt) {
	if !pkgutil.IsErrorType(d.analyzer.pass.TypesInfo.TypeOf(lhs)) {
		return
	}

	ident, ok := lhs.(*ast.Ident)
	if !ok {
		return
	}

	info := d.AnalyzeError(rhs)
	if info == nil {
		return
	}

	if d.scopes[scope] == nil {
		d.scopes[scope] = make(map[string]*ErrorInfo)
	}

	d.scopes[scope][ident.Name] = info
}

func (d *Detector) findInScope(name string) *ErrorInfo {
	for _, scopeMap := range d.scopes {
		if info, exists := scopeMap[name]; exists {
			return info
		}
	}
	return nil
}

func (d *Detector) CheckDepth(info *ErrorInfo) {
	if info == nil {
		return
	}

	if info.WrapDepth > d.analyzer.config.MaxWrapDepth {
		d.analyzer.reportIssue(info.WrapLocation, "max-depth-exceeded",
			"error wrapping depth %d exceeds maximum %d",
			info.WrapDepth, d.analyzer.config.MaxWrapDepth)
	}
}