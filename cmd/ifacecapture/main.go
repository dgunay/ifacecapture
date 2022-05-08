package main

import (
	"github.com/dgunay/ifacecapture/ifacecapture"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(ifacecapture.Analyzer)
}
