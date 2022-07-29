package testutils

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var update bool

// SaveAndLoadFromGolden loads the element from an yaml golden file in testdata/golden.
// It will update the file if the update flag is used prior to deserializing it.
func SaveAndLoadFromGolden[E any](t *testing.T, ref E) E {
	t.Helper()

	goldPath := filepath.Join("testdata", "golden", t.Name())
	// Update golden file
	if update {
		t.Logf("updating golden file %s", goldPath)
		err := os.MkdirAll(filepath.Dir(goldPath), 0755)
		require.NoError(t, err, "Cannot create directory for updating golden files")
		data, err := yaml.Marshal(ref)
		require.NoError(t, err, "Cannot marshal object to YAML")
		err = os.WriteFile(goldPath, data, 0600)
		require.NoError(t, err, "Cannot write golden file")
	}

	var want E
	data, err := os.ReadFile(goldPath)
	require.NoError(t, err, "Cannot load golden file")
	err = yaml.Unmarshal(data, &want)
	require.NoError(t, err, "Cannot create object from golden file")

	return want
}

// InstallUpdateFlag install an update flag referenced in this package.
// The flags need to be parsed before running the tests.
func InstallUpdateFlag() {
	flag.BoolVar(&update, "update", false, "update golden files")
}
