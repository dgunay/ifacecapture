package main

import (
	"github.com/dgunay/transaction-handle/transaction"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(transaction.PossiblyUnintentionalInterfaceCaptureAnalyzer)
}
