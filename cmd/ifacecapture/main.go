package main

import (
	"github.com/dgunay/ifacecapture/ifacecapture"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	analyzer := ifacecapture.Analyzer
	analyzer.Flags.StringVar(&ifacecapture.Loglvl, "loglvl", ifacecapture.Loglvl, "log level")
	analyzer.Flags.Var(&ifacecapture.InterfacesIgnoreList, "ignore-interfaces", "list of interfaces to ignore")
	analyzer.Flags.Var(&ifacecapture.InterfacesAllowList, "allow-interfaces", "list of interfaces to allow")
	singlechecker.Main(analyzer)
}
