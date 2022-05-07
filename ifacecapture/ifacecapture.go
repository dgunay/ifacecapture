package ifacecapture

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/tools/go/analysis"
)

var Analyzer *analysis.Analyzer = &analysis.Analyzer{
	Name: "ifacecapture",
	Doc:  "Checks for possibly unintentional captures of variables implementing an interface of a parameter in a callback function.",
	Run:  run,
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
	loglvl = "info"

	// Captured usages of types implementing interfaces on the ignore list will
	// not be reported.
	InterfacesIgnoreList arrayFlags = arrayFlags{}

	// If not empty, only captured usages of types implementing interfaces on
	// the allow list will not be reported.
	InterfacesAllowList arrayFlags = arrayFlags{}
)

func init() {
	Analyzer.Flags.StringVar(&loglvl, "loglvl", loglvl, "log level")
	Analyzer.Flags.Var(&InterfacesIgnoreList, "ignore-interfaces", "list of interfaces to ignore")
	Analyzer.Flags.Var(&InterfacesAllowList, "allow-interfaces", "list of interfaces to allow")
}

func run(pass *analysis.Pass) (any, error) {
	logger := logrus.New()

	lvl, err := logrus.ParseLevel(loglvl)
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
		capturedCalls := []CallViaReceiver{}
		ast.Inspect(callback.Body, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.CallExpr:
				capturedCall := NewCallViaReceiver()

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
			capturedType := pass.TypesInfo.TypeOf(capturedCall.Receiver())

			if !IsPointerType(capturedType) {
				// Prevents false negatives from captured variables that are
				// not pointers, but whose type does implement the interface.
				capturedType = types.NewPointer(capturedType)
			}

			for _, param := range paramInterfaceTypes {
				if !ShouldCheckInterface(param.Interface, InterfacesAllowList, InterfacesIgnoreList) {
					continue
				}

				ifaceType := pass.TypesInfo.TypeOf(param.Interface).Underlying().(*types.Interface)
				if types.Implements(capturedType, ifaceType) {
					pass.Reportf(
						capturedCall.Receiver().Pos(),
						"captured variable %s implements interface %s",
						capturedCall.String(), param.Interface.Name,
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
