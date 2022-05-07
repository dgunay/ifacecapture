package ifacecapture

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var PossiblyUnintentionalInterfaceCaptureAnalyzer *analysis.Analyzer = &analysis.Analyzer{
	Name: "ifacecapture",
	Doc:  "Checks for possibly unintentional captures of variables implementing an interface of a parameter in a callback function.",
	Run:  FindPossiblyUnintentionalInterfaceCaptures,
}

func FindPossiblyUnintentionalInterfaceCaptures(pass *analysis.Pass) (any, error) {
	inspect := func(node ast.Node) bool {
		// Step 1: is this node a function call?
		if !IsFunctionCall(node) {
			return true
		}

		// Step 2: is this a call to a function with at least one arg?
		callExpr := node.(*ast.CallExpr)
		if len(callExpr.Args) < 1 {
			return true
		}

		// Step 3: is the callback a function literal?
		callback, ok := callExpr.Args[0].(*ast.FuncLit)
		if !ok {
			return true
		}

		// Step 4: gather all interface types in the param list
		type ParamType struct {
			Vars      []*ast.Ident
			Interface *ast.Ident
		}
		var paramInterfaceTypes []ParamType
		for _, param := range callback.Type.Params.List {
			if param.Type != nil {
				vars := param.Names

				// Is it an interface?
				ident := param.Type.(*ast.Ident)
				typeSpec := ident.Obj.Decl.(*ast.TypeSpec)
				_, ok := typeSpec.Type.(*ast.InterfaceType)
				if ok {
					paramInterfaceTypes = append(paramInterfaceTypes, ParamType{
						Interface: ident,
						Vars:      vars,
					})
				}
			}
		}

		// Step 5: gather all captured variables in the body
		var capturedVariables []*ast.Ident
		ast.Inspect(callback.Body, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.Ident:
				// Is it a variable?
				ident := node.(*ast.Ident)
				if ident.Obj != nil && ident.Obj.Kind == ast.Var {
					// Was this declared outside the callback?
					if ident.Obj.Decl != nil {
						switch ident.Obj.Decl.(type) {
						case *ast.Field, *ast.AssignStmt:
							declPos := ident.Obj.Decl.(ast.Node).Pos()
							if declPos < callback.Pos() {
								capturedVariables = append(capturedVariables, ident)
							}
						}
					}
				}
			}
			return true
		})

		// Do any of them implement interfaces in the param list?
		for _, captured := range capturedVariables {
			capturedType := pass.TypesInfo.TypeOf(captured)

			if !IsPointerType(capturedType) {
				// Prevents false negatives from captured variables that are
				// not pointers, but whose type does implement the interface.
				capturedType = types.NewPointer(capturedType)
			}

			for _, param := range paramInterfaceTypes {
				ifaceType := pass.TypesInfo.TypeOf(param.Interface).Underlying().(*types.Interface)
				if types.Implements(capturedType, ifaceType) {
					pass.Reportf(
						captured.Pos(),
						"captured variable %s implements interface %s",
						captured.Name, param.Interface.Name,
					)
				}

			}
		}

		return false
	}

	for _, f := range pass.Files {
		ast.Inspect(f, inspect)
	}

	return nil, nil
}

func IsFunctionCall(node ast.Node) bool {
	switch node.(type) {
	case *ast.CallExpr:
		return true
	}
	return false
}

func IsFunctionLiteral(node ast.Node) bool {
	switch node.(type) {
	case *ast.FuncLit:
		return true
	}
	return false
}

func IsPointerType(t types.Type) bool {
	return strings.Contains(t.String(), "*")
}
