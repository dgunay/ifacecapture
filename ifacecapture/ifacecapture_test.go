package ifacecapture_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgunay/ifacecapture/ifacecapture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAll(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get wd: %s", err)
	}

	ifacecapture.Loglvl = "debug"

	testdata := filepath.Join(filepath.Dir(wd), "testdata")
	analysistest.Run(t, testdata, ifacecapture.Analyzer, "./src/...")
}
