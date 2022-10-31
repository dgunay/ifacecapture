package ifacecapture

import (
	"fmt"
	"go/ast"
)

// TypeChain traverses a chain of selector expressions like mypkg.MyInterface
// and collects the idents of the types in the chain.
type TypeChain struct {
	Types []*ast.Ident
}

func NewTypeChain() TypeChain {
	return TypeChain{Types: []*ast.Ident{}}
}

// ProcessTypeChain traverses a chain of selector expressions like
// mypkg.MyInterface and collects the idents of the types in the chain.
func (t *TypeChain) ProcessTypeChain(expr ast.Expr) error {
	switch expr := expr.(type) {
	case *ast.Ident:
		t.Types = append(t.Types, expr)
	case *ast.SelectorExpr:
		idents := []*ast.Ident{}
		err := traverseSelChain(&idents, expr)
		if err != nil {
			return err
		}

		// reverse the order of the idents
		for i, j := 0, len(idents)-1; i < j; i, j = i+1, j-1 {
			idents[i], idents[j] = idents[j], idents[i]
		}

		t.Types = append(t.Types, idents...)
	case *ast.StarExpr:
		return t.ProcessTypeChain(expr.X)
	}

	return nil
}

func traverseSelChain(idents *[]*ast.Ident, selExpr *ast.SelectorExpr) error {
	*idents = append(*idents, selExpr.Sel)
	switch expr := selExpr.X.(type) {
	case *ast.Ident:
		*idents = append(*idents, expr)
		return nil
	case *ast.SelectorExpr:
		return traverseSelChain(idents, expr)
	default:
		return fmt.Errorf("expected identifier, got %T: %w", selExpr.X, ErrUnexpectedType)
	}
}

// IsInterface returns true if the last element of the chain is an interface.
func (t TypeChain) IsInterface() bool {
	// last in the chain is an interface
	last := t.Last()

	obj := last.Obj
	if obj == nil {
		return false
	}
	typeSpec, ok := obj.Decl.(*ast.TypeSpec)
	if !ok {
		return false
	}

	_, ok = typeSpec.Type.(*ast.InterfaceType)
	return ok
}

// Last returns the last element in the chain. Panics if the chain is empty.
func (t TypeChain) Last() *ast.Ident {
	return t.Types[len(t.Types)-1]
}
