package ifacecapture

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
)

// CallViaReceiver represents a call to a function on a receiver, possibly
// through a chain of Selector expressions.
type CallViaReceiver struct {
	typeinfo *types.Info

	// The type of the receiver for this call
	ReceivedByType types.Type

	// The func that this call is done on
	Func *types.Func

	// The chain of Selector expressions that lead to the function. E.g. in
	// a.b.c.foo(), the selectors are [a, b, c].
	Chain []*ast.Ident
}

func NewCallViaReceiver(tinfo *types.Info) CallViaReceiver {
	return CallViaReceiver{
		typeinfo: tinfo,
		Chain:    []*ast.Ident{},
	}
}

var errSkip = errors.New("skip")

// ProcessSelExpr recursively processes a SelectorExpr and adds the chain of
// Idents to the .Chain field.
func (c *CallViaReceiver) ProcessSelExpr(expr *ast.SelectorExpr) error {
	if c.ReceivedByType == nil {
		sel := c.typeinfo.Selections[expr]
		if sel == nil {
			return fmt.Errorf("No type info for %s", expr)
		}

		c.ReceivedByType = sel.Recv()
		c.Func, _ = sel.Obj().(*types.Func)
	}

	switch x := expr.X.(type) {
	case *ast.Ident:
		c.Chain = append(c.Chain, x)
	case *ast.SelectorExpr:
		if x != nil {
			err := c.ProcessSelExpr(x)
			c.Chain = append(c.Chain, x.Sel)
			return err
		}
	case *ast.IndexExpr, *ast.CallExpr:
		return errSkip
	default:
		return fmt.Errorf("unexpected type for SelectorExpr.X: %T", expr.X)
	}

	return nil
}

func (c CallViaReceiver) Receiver() *ast.Ident {
	// last of the chain
	return c.Chain[len(c.Chain)-1]
}

// Formats the CallViaReceiver as a string in the form "a.b.c"
func (c CallViaReceiver) String() string {
	selChainString := ""
	sep := ""
	for _, sel := range c.Chain {
		selChainString += sep + sel.Name
		sep = "."
	}

	return selChainString
}
