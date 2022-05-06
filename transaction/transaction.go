package transaction

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var PossiblyUnintentionalInterfaceCaptureAnalyzer *analysis.Analyzer = &analysis.Analyzer{
	Name: "interfaceacpture",
	Doc:  "Checks for possibly unintentional captures of variables implementing an interface of a parameter in a callback function.",
	Run:  FindPossiblyUnintentionalInterfaceCaptures,
}

// TODO: https://disaev.me/p/writing-useful-go-analysis-linter/
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

		// TODO: iterate over all the callExpr args
		// Step 3: is the callback a function literal?
		callback, ok := callExpr.Args[0].(*ast.FuncLit)
		if !ok {
			return true
		}
		pass.Reportf(node.Pos(), "callback here")

		// Step 4: gather all interface types in the param list
		type ParamType struct {
			Vars []*ast.Ident
			Name string
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
						Name: ident.Name,
						Vars: vars,
					})
				}
			}
		}

		for _, param := range paramInterfaceTypes {
			pass.Reportf(callback.Pos(), "variables %v of interface type %s", param.Vars, param.Name)
		}

		// Step 5: gather all captured variables in the body
		// var capturedVariables []*ast.Ident
		// ast.Inspect(callback.Body, func(node ast.Node) bool {
		// 	if !IsFunctionLiteral(node) {
		// 		return true
		// 	}

		// Step 6: gather all variables in the body

		// Are there any captured variables that implement any of these
		// interfaces?
		// callback.Body.

		pass.Reportf(node.Pos(), "function literal")

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
