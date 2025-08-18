package pkgutil

import (
	"go/ast"
	"go/types"
	"strings"
)

func GetPackagePath(obj types.Object) string {
	if obj == nil {
		return ""
	}
	if obj.Pkg() != nil {
		return obj.Pkg().Path()
	}
	return ""
}

func IsExternalPackage(objPkg, currentPkg string) bool {
	if objPkg == "" || currentPkg == "" {
		return false
	}
	return objPkg != currentPkg
}

func IsStandardLibrary(pkg string) bool {
	if pkg == "" {
		return false
	}
	
	return !strings.Contains(pkg, ".") && !strings.Contains(pkg, "/vendor/")
}

func GetCallPackage(call *ast.CallExpr, info *types.Info) string {
	if call == nil || info == nil {
		return ""
	}

	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if ident, ok := fn.X.(*ast.Ident); ok {
			if obj := info.ObjectOf(ident); obj != nil {
				if pkgName, ok := obj.(*types.PkgName); ok {
					return pkgName.Imported().Path()
				}
			}
		}
		
		if obj := info.ObjectOf(fn.Sel); obj != nil {
			return GetPackagePath(obj)
		}
	case *ast.Ident:
		if obj := info.ObjectOf(fn); obj != nil {
			return GetPackagePath(obj)
		}
	}
	
	return ""
}

func GetErrorOriginPackage(expr ast.Expr, info *types.Info) string {
	if expr == nil || info == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if obj := info.ObjectOf(e); obj != nil {
			return GetPackagePath(obj)
		}
	case *ast.CallExpr:
		return GetCallPackage(e, info)
	case *ast.SelectorExpr:
		if obj := info.ObjectOf(e.Sel); obj != nil {
			return GetPackagePath(obj)
		}
	}
	
	return ""
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

func HasUnwrapMethod(typ types.Type) bool {
	if typ == nil {
		return false
	}

	methods := types.NewMethodSet(typ)
	for i := 0; i < methods.Len(); i++ {
		method := methods.At(i).Obj()
		if method.Name() == "Unwrap" {
			sig, ok := method.Type().(*types.Signature)
			if !ok {
				continue
			}
			
			if sig.Params().Len() == 0 && sig.Results().Len() == 1 {
				result := sig.Results().At(0).Type()
				if IsErrorType(result) {
					return true
				}
			}
		}
	}
	
	if named, ok := typ.(*types.Named); ok {
		return HasUnwrapMethod(named.Underlying())
	}
	
	return false
}

func ImplementsError(typ types.Type) bool {
	if typ == nil {
		return false
	}

	methods := types.NewMethodSet(typ)
	for i := 0; i < methods.Len(); i++ {
		method := methods.At(i).Obj()
		if method.Name() == "Error" {
			sig, ok := method.Type().(*types.Signature)
			if !ok {
				continue
			}
			
			if sig.Params().Len() == 0 && sig.Results().Len() == 1 {
				result := sig.Results().At(0).Type()
				if basic, ok := result.(*types.Basic); ok && basic.Kind() == types.String {
					return true
				}
			}
		}
	}
	
	return false
}