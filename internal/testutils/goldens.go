package testutils

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type goldenOption struct {
	goldPath string
}

// OptionGolden is a supported option reference to change the golden files comparison.
type OptionGolden func(*goldenOption)

// WithGoldPath overrides the default path for golden files used.
func WithGoldPath(path string) OptionGolden {
	return func(o *goldenOption) {
		if path != "" {
			o.goldPath = path
		}
	}
}

var update bool

// LoadAndUpdateYAMLFromGolden loads the element from an yaml golden file in testdata/golden.
// It will update the file if the update flag is used prior to deserializing it.
func LoadAndUpdateYAMLFromGolden[E any](t *testing.T, ref E, opts ...OptionGolden) E {
	t.Helper()

	o := goldenOption{
		goldPath: filepath.Join("testdata", "golden", t.Name()),
	}

	for _, opt := range opts {
		opt(&o)
	}

	// Update golden file
	if update {
		t.Logf("updating golden file %s", o.goldPath)
		err := os.MkdirAll(filepath.Dir(o.goldPath), 0750)
		require.NoError(t, err, "Cannot create directory for updating golden files")
		data, err := yaml.Marshal(ref)
		require.NoError(t, err, "Cannot marshal object to YAML")
		err = os.WriteFile(o.goldPath, data, 0600)
		require.NoError(t, err, "Cannot write golden file")
	}

	var want E
	data, err := os.ReadFile(o.goldPath)
	require.NoError(t, err, "Cannot load golden file")
	err = yaml.Unmarshal(data, &want)
	require.NoError(t, err, "Cannot create object from golden file")

	return want
}

// SaveAndLoadFromGolden loads the element from a plaintext golden file in testdata/golden.
// It will update the file if the update flag is used prior to deserializing it.
func SaveAndLoadFromGolden(t *testing.T, data string) string {
	t.Helper()

	goldPath := filepath.Join("testdata", "golden", t.Name())

	if update {
		t.Logf("updating golden file %s", goldPath)
		err := os.MkdirAll(filepath.Dir(goldPath), 0750)
		require.NoError(t, err, "Cannot create directory for updating golden files")
		err = os.WriteFile(goldPath, []byte(data), 0600)
		require.NoError(t, err, "Cannot write golden file")
	}

	want, err := os.ReadFile(goldPath)
	require.NoError(t, err, "Cannot load golden file")

	return string(want)
}

// InstallUpdateFlag install an update flag referenced in this package.
// The flags need to be parsed before running the tests.
func InstallUpdateFlag() {
	flag.BoolVar(&update, "update", false, "update golden files")
}
