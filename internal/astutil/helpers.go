package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

func FindFormatString(call *ast.CallExpr) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}
	
	if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.Trim(lit.Value, `"`), true
	}
	
	return "", false
}

func HasPercentW(formatStr string) bool {
	return strings.Contains(formatStr, "%w")
}

func HasPercentV(formatStr string) bool {
	return strings.Contains(formatStr, "%v")
}

func IsNilExpr(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "nil"
	}
	return false
}

func GetIdentObject(ident *ast.Ident, info *types.Info) types.Object {
	if ident == nil || info == nil {
		return nil
	}
	return info.ObjectOf(ident)
}

func GetExprType(expr ast.Expr, info *types.Info) types.Type {
	if expr == nil || info == nil {
		return nil
	}
	return info.TypeOf(expr)
}

func IsFunctionCall(expr ast.Expr, pkg, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	
	if sel.Sel.Name != name {
		return false
	}
	
	if pkg == "" {
		return true
	}
	
	if ident, ok := sel.X.(*ast.Ident); ok {
		return ident.Name == pkg
	}
	
	return false
}

func ExtractErrorFromReturn(ret *ast.ReturnStmt, errorPos int) ast.Expr {
	if ret == nil || errorPos < 0 || errorPos >= len(ret.Results) {
		return nil
	}
	return ret.Results[errorPos]
}

func FindErrorPositionInSignature(sig *types.Signature) int {
	if sig == nil || sig.Results() == nil {
		return -1
	}
	
	for i := 0; i < sig.Results().Len(); i++ {
		typ := sig.Results().At(i).Type()
		if IsErrorType(typ) {
			return i
		}
	}
	
	return -1
}

func IsErrorType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	
	errorType := types.Universe.Lookup("error").Type()
	if errorType == nil {
		return false
	}
	
	return types.Implements(typ, errorType.Underlying().(*types.Interface)) ||
		types.Identical(typ, errorType)
}

func GetFunctionSignature(fn ast.Expr, info *types.Info) *types.Signature {
	if fn == nil || info == nil {
		return nil
	}
	
	typ := info.TypeOf(fn)
	if typ == nil {
		return nil
	}
	
	if sig, ok := typ.(*types.Signature); ok {
		return sig
	}
	
	if sig, ok := typ.Underlying().(*types.Signature); ok {
		return sig
	}
	
	return nil
}

func IsTypeAssertion(expr ast.Expr) bool {
	_, ok := expr.(*ast.TypeAssertExpr)
	return ok
}

func IsTypeSwitch(stmt ast.Stmt) bool {
	if typeSwitch, ok := stmt.(*ast.TypeSwitchStmt); ok {
		return typeSwitch != nil
	}
	return false
}

func ContainsErrorCheck(block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}
	
	for _, stmt := range block.List {
		if ifStmt, ok := stmt.(*ast.IfStmt); ok {
			if binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr); ok {
				if binExpr.Op == token.NEQ || binExpr.Op == token.EQL {
					if IsNilExpr(binExpr.X) || IsNilExpr(binExpr.Y) {
						return true
					}
				}
			}
		}
	}
	
	return false
}

func TraceErrorSource(expr ast.Expr, info *types.Info, visited map[ast.Expr]bool) ast.Expr {
	if visited == nil {
		visited = make(map[ast.Expr]bool)
	}
	
	if visited[expr] {
		return expr
	}
	visited[expr] = true
	
	switch e := expr.(type) {
	case *ast.Ident:
		obj := info.ObjectOf(e)
		if obj == nil {
			return expr
		}
		
		if varObj, ok := obj.(*types.Var); ok {
			if assign := findAssignment(varObj); assign != nil {
				return TraceErrorSource(assign, info, visited)
			}
		}
		return expr
		
	case *ast.CallExpr:
		return expr
		
	case *ast.SelectorExpr:
		return TraceErrorSource(e.X, info, visited)
		
	default:
		return expr
	}
}

func findAssignment(v *types.Var) ast.Expr {
	return nil
}