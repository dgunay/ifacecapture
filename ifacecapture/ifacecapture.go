package ifacecapture

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"github.com/dgunay/ifacecapture/ifacecapture/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var Analyzer *analysis.Analyzer = &analysis.Analyzer{
	Name: "ifacecapture",
	Doc:  "Checks for possibly unintentional captures of variables implementing an interface of a parameter in a callback function.",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
}

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// Flags
var (
	Loglvl = "info"

	// Captured usages of types implementing interfaces on the ignore list will
	// not be reported.
	InterfacesIgnoreList arrayFlags = arrayFlags{}

	// If not empty, only captured usages of types implementing interfaces on
	// the allow list will not be reported.
	InterfacesAllowList arrayFlags = arrayFlags{}
)

type ParamType struct {
	Vars           []*ast.Ident
	InterfaceIdent *ast.Ident
	InterfaceType  *types.Interface
}

func run(pass *analysis.Pass) (any, error) {
	logger := logrus.New()

	lvl, err := logrus.ParseLevel(Loglvl)
	if err != nil {
		return false, err
	}
	logger.SetLevel(lvl)

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

		// Step 3: is the last argument to the call expression a function literal?
		callback, ok := callExpr.Args[len(callExpr.Args)-1].(*ast.FuncLit)
		if !ok {
			return true
		}

		logger.Debugf("Examining function %s with callback", renderSafe(pass.Fset, callExpr.Fun))

		// Figure out left side of the call. If it's a selection expr like a.b.Foo(), we'll note down the type of `a.b`.
		var receiverType types.Type
		if leftSide, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			logger.Debugf("left: %v", leftSide)
			call := NewCallViaReceiver(pass.TypesInfo)
			err := call.ProcessSelExpr(leftSide)
			if err != nil {
				logger.Debugf("converting left expr: %s", err)
			}

			logger.Debugf("call on left side: %+v", call.ReceivedByType)
			receiverType = call.ReceivedByType
		}

		// If any of the callback parameters matches the receiver type, verify that the only references to a value of that type
		// are to callback parameters, not to the receiver.

		// Step 4: gather all interface types in the param list

		var (
			paramInterfaceTypes []ParamType
			paramVars           []*ast.Ident
			checkReceiverCalls  bool
		)
		for _, param := range callback.Type.Params.List {
			if param.Type != nil {
				vars := param.Names

				paramType := pass.TypesInfo.TypeOf(param.Type)
				logger.Debugf("param %s, type: %s", vars, paramType)

				// One of the parameters has the same type as the receiver. This usually means a method that gets a child
				// of some data structure as its parameter, for example in (*testing.T).Run's 2nd parameter.
				if receiverType != nil && paramType != nil && paramType.String() == receiverType.String() {
					checkReceiverCalls = true
				}

				paramVars = append(paramVars, vars...)

				if _, ok := paramType.Underlying().(*types.Interface); ok {
					// May have to go forward all the way to get the ident
					chain := NewTypeChain()
					if err := chain.ProcessTypeChain(param.Type); err != nil {
						logger.Errorf("Failed to process type chain: %s", err)
						continue
					}

					paramInterfaceTypes = append(paramInterfaceTypes, ParamType{
						InterfaceIdent: chain.Last(),
						InterfaceType:  paramType.Underlying().(*types.Interface),
						Vars:           vars,
					})
				}
			}
		}

		if len(paramInterfaceTypes) == 0 && !checkReceiverCalls {
			logger.Debug("No interfaces found in param list, and not checking for method receiver calls")
			return true
		}
		logger.Debugf("Found interfaces %v in param list of %s", paramInterfaceTypes, renderSafe(pass.Fset, callback.Type))
		logger.Debugf("Checking receiver types: %t", checkReceiverCalls)

		// Step 5: gather all captured variables in the body
		// Get all CallExprs with receivers
		capturedCalls := []CallViaReceiver{}
		ast.Inspect(callback.Body, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.CallExpr:
				capturedCall := NewCallViaReceiver(pass.TypesInfo)

				expr := node.(*ast.CallExpr).Fun
				if selExpr, ok := expr.(*ast.SelectorExpr); ok {
					err := capturedCall.ProcessSelExpr(selExpr)
					if err == nil {
						capturedCalls = append(capturedCalls, capturedCall)
					} else {
						logger.Error(err)
					}
				}
			}
			return true
		})

		// Do any of them implement interfaces in the param list?
		for _, capturedCall := range capturedCalls {
			capturedType := capturedCall.ReceivedByType
			logger.Debugf("Examining captured type %v", capturedType)

			// A call on the receiver is invalid if a value of the same type exist in the parameter list, and
			// the value in the parameter list does not have the same name as the receiver (because then
			// it'll shadow the receiver)

			invalidReceiverCall := checkReceiverCalls && capturedType.String() == receiverType.String()
			invalidReceiverCall = invalidReceiverCall && !util.Any(paramVars, func(pv *ast.Ident) bool {
				if capturedCall.Receiver().Name == pv.Name {
					return true
				}
				return capturedCall.Receiver().Obj == pv.Obj
			})

			if invalidReceiverCall {
				logger.Debugf("seen invalid receiver call")
				ReportReceiverCall(pass, &capturedCall)
			}

			for _, paramType := range paramInterfaceTypes {
				// Don't check if the receiver is one of the function params, or it is shadowed by one of
				// the parameters.
				if util.Any(paramType.Vars, func(paramVar *ast.Ident) bool {
					if capturedCall.Receiver().Name == paramVar.Name {
						return true
					}

					return capturedCall.Receiver().Obj == paramVar.Obj
				}) {
					logger.Debugf("Skipping captured type %v because it is a param", capturedType)
					continue
				}

				if !ShouldCheckInterface(paramType.InterfaceIdent, InterfacesAllowList, InterfacesIgnoreList) {
					continue
				}

				ifaceType := paramType.InterfaceType
				logger.Debugf("Checking if %s implements %s", capturedType, paramType.InterfaceIdent.Name)

				// FIXME: it is unclear to me why sometimes it is necessary
				// to convert the type to a pointer before checking if it
				// implements the interface. Haven't yet reproduced the bug.
				if types.Implements(capturedType, ifaceType) || types.Implements(types.NewPointer(capturedType), ifaceType) {
					// Figure out if the method being called in capturedCall is part of ifaceType's method set.
					// If it's not, we're looking at a type assertion for an embedded interface and shouldn't
					// report.
					var doReport bool
					if capturedCall.Func != nil {
						for i := 0; i < ifaceType.NumMethods(); i++ {
							if capturedCall.Func.FullName() == ifaceType.Method(i).FullName() {
								doReport = true
							}
						}
					}

					if doReport {
						Report(pass, &capturedCall, paramType)
					}
				}

			}
		}

		return false
	}

	logger.Debugf("pkg: %+v", pass.Pkg.Path())

	for _, f := range pass.Files {
		logger.Debugf("Examining package %s", f.Name)
		ast.Inspect(f, inspect)
	}

	return nil, nil
}

func Report(pass *analysis.Pass, call *CallViaReceiver, paramType ParamType) {
	identPackage := pass.TypesInfo.ObjectOf(paramType.InterfaceIdent).Pkg()
	identString := ""
	if pass.Pkg != identPackage {
		identString = fmt.Sprintf("%s.", identPackage.Name())
	}
	identString += paramType.InterfaceIdent.Name

	pass.Reportf(
		call.Receiver().Pos(),
		"captured variable %s implements interface %s",
		call.String(), identString,
	)
}

func ReportReceiverCall(pass *analysis.Pass, call *CallViaReceiver) {
	pass.Reportf(
		call.Receiver().Pos(),
		"method call on receiver type %s not through parameter",
		call.String(),
	)
}

func ShouldCheckInterface(iface *ast.Ident, allowList, ignoreList []string) bool {
	if iface == nil {
		return false
	}

	ifaceName := iface.Name

	if len(allowList) > 0 {
		for _, allow := range allowList {
			if allow == ifaceName {
				return true
			}
		}
	}

	if len(ignoreList) > 0 {
		for _, ignore := range ignoreList {
			if ignore == ifaceName {
				return false
			}
		}
	}

	return true
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

// render returns the pretty-print of the given node
func render(fset *token.FileSet, x interface{}) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, x); err != nil {
		return "", fmt.Errorf("render: %s", err)
	}
	return buf.String(), nil
}

func renderSafe(fset *token.FileSet, x interface{}) string {
	str, err := render(fset, x)
	if err != nil {
		return fmt.Sprintf("%T", x)
	}

	return str
}
