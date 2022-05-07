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
		// Get all CallExprs with receivers
		type CapturedCall struct {
			Selectors []*ast.Ident
			Receiver  *ast.Ident
		}

		capturedCalls := []CapturedCall{}
		ast.Inspect(callback.Body, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.CallExpr:
				capturedCall := CapturedCall{Selectors: []*ast.Ident{}}

				callExpr := node.(*ast.CallExpr)

				// FIXME: we're not trying to do anything conceptually hard, this
				// algorithm just looks terrible because I am not great at using
				// Go's AST inspection API yet. Simplify it.
				// A call will look like this:
				// ident1.ident2.theMethod(...)
				// We start from x: ident2 and sel: theMethod
				// then it will be x: ident1 and sel: ident2
				// We want to capture ident2 as an Ident since it is the receiver,
				// but save the rest of the chain as a string for diagnostic
				// purposes.
				expr := callExpr.Fun
				endOfSelChain := false
				for !endOfSelChain {
					// processSelExpr(&capturedCall, expr.(*ast.SelectorExpr))
					if selExpr, ok := expr.(*ast.SelectorExpr); ok {
						if capturedCall.Receiver == nil {
							if receiver, ok := selExpr.X.(*ast.Ident); ok {
								capturedCall.Receiver = receiver
							} else if selExpr.X.(*ast.SelectorExpr) != nil {
								capturedCall.Receiver = selExpr.X.(*ast.SelectorExpr).Sel
								selExpr.X = selExpr.X.(*ast.SelectorExpr).X // Skip one since we took it as the receiver
							}
						} else {
							capturedCall.Selectors = append(capturedCall.Selectors, selExpr.Sel)
						}
						expr = selExpr.X
					} else if ident, ok := expr.(*ast.Ident); ok {
						if ident != capturedCall.Receiver {
							capturedCall.Selectors = append(capturedCall.Selectors, ident)
						}
						endOfSelChain = true
					} else {
						panic("unexpected")
					}
				}
				capturedCalls = append(capturedCalls, capturedCall)
			}
			return true
		})

		// TODO: uncomment
		// Do any of them implement interfaces in the param list?
		for _, capturedCall := range capturedCalls {
			capturedType := pass.TypesInfo.TypeOf(capturedCall.Receiver)

			if !IsPointerType(capturedType) {
				// Prevents false negatives from captured variables that are
				// not pointers, but whose type does implement the interface.
				capturedType = types.NewPointer(capturedType)
			}

			for _, param := range paramInterfaceTypes {
				ifaceType := pass.TypesInfo.TypeOf(param.Interface).Underlying().(*types.Interface)
				if types.Implements(capturedType, ifaceType) {
					selChainString := ""
					for _, sel := range capturedCall.Selectors {
						selChainString += sel.Name + "."
					}
					pass.Reportf(
						capturedCall.Receiver.Pos(),
						"captured variable %s%s implements interface %s",
						selChainString, capturedCall.Receiver.Name, param.Interface.Name,
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
