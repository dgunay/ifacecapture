package main

import (
	"github.com/dgunay/transaction-handle/ifacecapture"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(ifacecapture.Analyzer)
}
