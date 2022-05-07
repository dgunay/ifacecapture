package ifacecapture

import (
	"fmt"
	"go/ast"
)

// CallViaReceiver represents a call to a function on a receiver, possibly
// through a chain of Selector expressions.
type CallViaReceiver struct {
	// The chain of Selector expressions that lead to the function. E.g. in
	// a.b.c.foo(), the selectors are [a, b, c].
	Chain []*ast.Ident
}

func NewCallViaReceiver() CallViaReceiver {
	return CallViaReceiver{
		Chain: []*ast.Ident{},
	}
}

// ProcessSelExpr recursively processes a SelectorExpr and adds the chain of
// Idents to the .Chain field.
func (c *CallViaReceiver) ProcessSelExpr(expr *ast.SelectorExpr) error {
	if ident, ok := expr.X.(*ast.Ident); ok {
		c.Chain = append(c.Chain, ident)
	} else if selExpr, ok := expr.X.(*ast.SelectorExpr); selExpr != nil && ok {
		err := c.ProcessSelExpr(selExpr)
		c.Chain = append(c.Chain, selExpr.Sel)
		return err
	} else {
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
