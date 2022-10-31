package ifacecapture

import (
	"bytes"
	"flag"
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

//nolint:gochecknoglobals
var Analyzer *analysis.Analyzer = &analysis.Analyzer{
	Name: "ifacecapture",
	Doc: "Checks for possibly unintentional captures of variables implementing " +
		"an interface of a parameter in a callback function.",
	Run: run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
	Flags:            flag.FlagSet{},
	RunDespiteErrors: false,
	ResultType:       nil,
	FactTypes:        nil,
}

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// Flags.
//
//nolint:gochecknoglobals
var (
	Loglvl = "info"

	// Captured usages of types implementing interfaces on the ignore list will
	// not be reported.
	InterfacesIgnoreList = arrayFlags{}

	// If not empty, only captured usages of types implementing interfaces on
	// the allow list will not be reported.
	InterfacesAllowList = arrayFlags{}
)

type InterfaceParamType struct {
	Vars           []*ast.Ident
	InterfaceIdent *ast.Ident
	InterfaceType  *types.Interface
}

type ConcreteParamType struct {
	Vars  []*ast.Ident
	Ident *ast.Ident
	Type  types.Type
}

func run(pass *analysis.Pass) (any, error) {
	logger := logrus.New()

	lvl, err := logrus.ParseLevel(Loglvl)
	if err != nil {
		return false, fmt.Errorf("parse log level %s: %w", Loglvl, err)
	}

	logger.SetLevel(lvl)

	inspect := func(node ast.Node) bool {
		// Step 1: is this node a function call?
		if !IsFunctionCall(node) {
			return true
		}

		// Step 2: is this a call to a function with at least one arg?
		callExpr, ok := node.(*ast.CallExpr)
		if !ok || len(callExpr.Args) < 1 {
			return true
		}

		// Step 3: is the callback a function literal?
		callback, ok := callExpr.Args[0].(*ast.FuncLit)
		if !ok {
			return true
		}

		logger.Debugf("Examining function %s with callback", renderSafe(pass.Fset, callExpr.Fun))

		// Step 4: gather all types in the param list
		paramInterfaceTypes, paramConcreteTypes := getTypesInParamList(pass, logger, callback)

		if len(paramInterfaceTypes) == 0 && len(paramConcreteTypes) == 0 {
			logger.Debug("No interfaces or concrete types found in param list")
			return true
		}
		logger.Debugf("Found interfaces %v in param list of %s", paramInterfaceTypes, renderSafe(pass.Fset, callback.Type))
		logger.Debugf("Found concrete types %v in param list of %s", paramConcreteTypes, renderSafe(pass.Fset, callback.Type))

		// Step 5: gather all captured variables in the body
		// Get all CallExprs with receivers
		capturedCalls := []CallViaReceiver{}
		ast.Inspect(callback.Body, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.CallExpr:
				capturedCall := NewCallViaReceiver(pass.TypesInfo)

				callExpr, ok := node.(*ast.CallExpr)
				if !ok {
					logger.Warnf("Could not cast %s to *ast.CallExpr", node)
					return true
				}
				expr := callExpr.Fun
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
		// Do any of them match the concrete types in the param list?
		for _, capturedCall := range capturedCalls {
			capturedCall := capturedCall
			capturedType := capturedCall.ReceivedByType
			logger.Debugf("Examining captured type %v", capturedType)

			for _, paramType := range paramInterfaceTypes {
				// Don't check if the receiver is one of the function params
				if util.Any(paramType.Vars, func(paramVar *ast.Ident) bool {
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
				if types.Implements(capturedType, ifaceType) {
					ReportInterface(pass, &capturedCall, paramType)
				} else if types.Implements(types.NewPointer(capturedType), ifaceType) {
					// NOTE: it is unclear to me why sometimes it is necessary
					// to convert the type to a pointer before checking if it
					// implements the interface. Haven't yet reproduced the bug.
					ReportInterface(pass, &capturedCall, paramType)
				}
			}

			for _, paramType := range paramConcreteTypes {
				// Don't check if the receiver is one of the function params
				if util.Any(paramType.Vars, func(paramVar *ast.Ident) bool {
					return capturedCall.Receiver().Obj == paramVar.Obj
				}) {
					logger.Debugf("Skipping captured type %v because it is a param", capturedType)
					continue
				}

				if paramType.Type == capturedType {
					ReportConcrete(pass, &capturedCall, paramType)
				}
			}
		}

		return false
	}

	for _, f := range pass.Files {
		logger.Debugf("Examining package %s", f.Name)
		ast.Inspect(f, inspect)
	}

	return nil, nil
}

// Given a function literal, return lists of the interface types and concrete
// types in the param list.
func getTypesInParamList(
	pass *analysis.Pass,
	logger *logrus.Logger,
	callback *ast.FuncLit,
) ([]InterfaceParamType, []ConcreteParamType) {
	var paramInterfaceTypes []InterfaceParamType
	var paramConcreteTypes []ConcreteParamType
	for _, param := range callback.Type.Params.List {
		if param.Type == nil {
			continue
		}

		vars := param.Names

		paramType := pass.TypesInfo.TypeOf(param.Type)
		underlying := paramType.Underlying()
		if IsPointerType(underlying) {
			asPointer, ok := underlying.(*types.Pointer)
			if !ok {
				logger.Warnf("Could not cast %s to *types.Pointer", underlying)
				continue
			}
			underlying = asPointer.Elem()
		}
		switch paramType := underlying.(type) {
		case *types.Interface, *types.Named:
			// May have to go forward all the way to get the ident
			chain := NewTypeChain()
			if err := chain.ProcessTypeChain(param.Type); err != nil {
				logger.Errorf("Failed to process type chain: %s", err)
				continue
			}

			if _, ok := paramType.(*types.Interface); ok {
				underlyingInterface, ok := paramType.Underlying().(*types.Interface)
				if !ok {
					logger.Warnf("Could not cast %s to *types.Interface", paramType)
					continue
				}
				paramInterfaceTypes = append(paramInterfaceTypes, InterfaceParamType{
					InterfaceIdent: chain.Last(),
					InterfaceType:  underlyingInterface,
					Vars:           vars,
				})
			} else {
				paramConcreteTypes = append(paramConcreteTypes, ConcreteParamType{
					Ident: chain.Last(),
					Type:  paramType,
					Vars:  vars,
				})
			}
		}
	}

	return paramInterfaceTypes, paramConcreteTypes
}

// ReportInterface reports that `call` implements the interface `iParamType`.
func ReportInterface(pass *analysis.Pass, capturedCall *CallViaReceiver, iParamType InterfaceParamType) {
	identPackage := pass.TypesInfo.ObjectOf(iParamType.InterfaceIdent).Pkg()
	identString := ""
	if pass.Pkg != identPackage {
		identString = fmt.Sprintf("%s.", identPackage.Name())
	}
	identString += iParamType.InterfaceIdent.Name

	pass.Reportf(
		capturedCall.Receiver().Pos(),
		"captured variable %s implements interface %s",
		capturedCall.String(), identString,
	)
}

// ReportConcrete reports that the receiver of `capturedCall` is the same concrete
// type as the type of `paramType`.
func ReportConcrete(pass *analysis.Pass, capturedCall *CallViaReceiver, paramType ConcreteParamType) {
	identString := paramType.Vars[0].Name

	pass.Reportf(
		capturedCall.Receiver().Pos(),
		"captured variable %s is of same type as parameter %s",
		capturedCall.String(), identString,
	)
}

// ShouldCheckInterface returns true if the given identifier for an interface
// is in `allowlist`, or not in `ignoreList`.
func ShouldCheckInterface(iface *ast.Ident, allowList, ignoreList []string) bool {
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

// IsFunctionCall returns true if the given expression is of type `ast.CallExpr`.
func IsFunctionCall(node ast.Node) bool {
	_, ok := node.(*ast.CallExpr)
	return ok
}

// IsFunctionLiteral returns true if the given expression is of type `ast.FuncLit`.
func IsFunctionLiteral(node ast.Node) bool {
	_, ok := node.(*ast.FuncLit)
	return ok
}

// IsFunctionDeclaration returns true if the given type, when stringified,
// contains a '*'.
func IsPointerType(t types.Type) bool {
	return strings.Contains(t.String(), "*")
}

// render returns the pretty-print of the given node.
func render(fset *token.FileSet, x interface{}) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, x); err != nil {
		return "", fmt.Errorf("render: %w", err)
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
