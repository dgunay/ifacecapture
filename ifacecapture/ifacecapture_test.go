package ifacecapture_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgunay/ifacecapture/ifacecapture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAll(t *testing.T) {
	t.Parallel()

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get wd: %s", err)
	}

	ifacecapture.Loglvl = "debug"

	testdata := filepath.Join(filepath.Dir(workDir), "testdata")
	analysistest.Run(t, testdata, ifacecapture.Analyzer, "./src/...")
}
