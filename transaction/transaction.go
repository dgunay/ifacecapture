package transaction

import "golang.org/x/tools/go/analysis"

var PossiblyUnintentionalInterfaceCaptureAnalyzer *analysis.Analyzer = &analysis.Analyzer{
	Name: "interfaceacpture",
	Doc:  "Checks for possibly unintentional captures of variables implementing an interface of a parameter in a callback function.",
	Run:  FindPossiblyUnintentionalInterfaceCaptures,
}

// TODO: https://disaev.me/p/writing-useful-go-analysis-linter/
func FindPossiblyUnintentionalInterfaceCaptures(pass *analysis.Pass) {
	panic("not implemented")
}
