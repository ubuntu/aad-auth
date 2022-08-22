package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/cmd/aad-cli/cli"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestConfigPrint(t *testing.T) {
	tests := map[string]struct {
		configFile string
		domain     string

		wantErr bool
	}{
		"default domain": {},
		"custom domain":  {domain: "example.com"},
		"type mismatch":  {configFile: "type-mismatch"},

		// error cases
		"missing required entries": {configFile: "missing-required", wantErr: true},
		"non-existent config":      {configFile: "non-existent", wantErr: true},
		"malformed config":         {configFile: "malformed", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			cmdArgs := []string{"config", "print"}

			if tc.domain != "" {
				cmdArgs = append(cmdArgs, "--domain", tc.domain)
			}

			if tc.configFile == "" {
				tc.configFile = filepath.Join("testdata", "aad.conf")
			} else {
				tc.configFile = filepath.Join("testdata", tc.configFile+".conf")
			}

			c := cli.New(cli.WithConfigFile(tc.configFile))
			got, err := runApp(t, c, cmdArgs...)

			if tc.wantErr {
				require.Error(t, err, "expected command to return an error")
				return
			}
			require.NoError(t, err, "expected command to succeed")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestConfigEdit(t *testing.T) {
	requiredConfig := "tenant_id = something\napp_id = something"
	badConfig := "tenant_id = something"
	malformedConfig := "aaaaaaaaaaaaa"

	tests := map[string]struct {
		configFile string
		newConfig  string

		wantErr       bool
		wantEditorErr bool
	}{
		"loads the previous config":                          {},
		"creates an empty config if previous is not present": {configFile: "nonexistent", newConfig: requiredConfig},

		// error cases
		"editor returns an error":         {wantEditorErr: true, wantErr: true},
		"cfg validation returns an error": {newConfig: badConfig, wantErr: true},
		"cfg loading returns an error":    {newConfig: malformedConfig, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.configFile == "" {
				tc.configFile = filepath.Join("testdata", "aad.conf")
			} else {
				tc.configFile = filepath.Join("testdata", tc.configFile+".conf")
			}

			// Copy the config file to a temporary location, so that changes do
			// not persist across tests.
			tc.configFile = copyToTempPath(t, tc.configFile)
			editorMock := newEditorMock(t, tc.configFile, tc.newConfig, tc.wantEditorErr)

			c := cli.New(cli.WithConfigFile(tc.configFile), cli.WithEditor(editorMock))
			got, err := runApp(t, c, "config", "edit")

			tempConfigPath := tempConfigPathFromOutput(t, got)
			if tc.wantErr {
				require.Error(t, err, "expected command to return an error")
				require.FileExists(t, tempConfigPath, "expected temporary config file to be present")
				return
			}
			require.NoError(t, err, "expected command to succeed")
			require.NoFileExists(t, tempConfigPath, "expected temporary config file not to be present")

			got = sanitizeTempPaths(t, got)
			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestConfigEditor(t *testing.T) {
	// Default behavior
	err := os.Unsetenv("EDITOR")
	require.NoError(t, err, "failed to unset EDITOR")

	c := cli.New()
	require.Equal(t, "nano", c.Editor(), "expected default editor to be nano")

	// Custom editor
	err = os.Setenv("EDITOR", "vim")
	require.NoError(t, err, "failed to set EDITOR")

	c = cli.New()
	require.Equal(t, "vim", c.Editor(), "expected editor to be vim")
}

// newEditorMock returns the path to a shell script that overrides the default
// config editor.
// The script prints the previous config file, and if a new config is provided,
// it replaces the previous config file with the new contents.
func newEditorMock(t *testing.T, configFile, newConfig string, wantErr bool) string {
	t.Helper()

	editor, err := os.CreateTemp(os.TempDir(), "editor-mock.*.sh")
	require.NoError(t, err, "failed to create temporary file")
	defer editor.Close()
	t.Cleanup(func() { os.Remove(editor.Name()) })

	var b bytes.Buffer
	b.WriteString("#!/bin/sh\n")

	b.WriteString(`echo "TEMPORARY CONFIG PATH: $1"`)
	// Exit early with an error if requested
	if wantErr {
		b.WriteString("\nexit 1\n")
	}

	b.WriteString(fmt.Sprintf(`
# Print previous config file
echo "PREVIOUS CONFIG FILE:"
cat %s

`, configFile))

	if newConfig != "" {
		b.WriteString(fmt.Sprintf(`# Update with new config contents
echo "NEW CONFIG FILE:"
echo "%s" | tee $1
`, newConfig))
	}

	err = editor.Chmod(0700)
	require.NoError(t, err, "failed to set executable permissions on temporary file")

	_, err = editor.Write(b.Bytes())
	require.NoError(t, err, "failed to write temporary file")

	return editor.Name()
}

// copyToTempPath copies the given config file to a temporary location, and returns the path to it.
// If the given config file is not present, a non-existent temporary path is returned.
func copyToTempPath(t *testing.T, file string) string {
	t.Helper()

	r, err := os.Open(file)
	if err != nil {
		// We assume a non-existent file was a deliberate choice,
		// so return back a non-existent file in a temporary directory.
		t.Cleanup(func() { os.Remove(filepath.Join(os.TempDir(), filepath.Base(file))) })
		return filepath.Join(os.TempDir(), filepath.Base(file))
	}
	defer r.Close()

	w, err := os.CreateTemp(os.TempDir(), "aad.*.conf")
	require.NoError(t, err, "failed to create temporary file")
	defer w.Close()
	t.Cleanup(func() { os.Remove(w.Name()) })

	_, err = w.ReadFrom(r)
	require.NoError(t, err, "failed to copy file")

	return w.Name()
}

// tempConfigPathFromOutput returns the path to the temporary config file used
// by the editor from the given string.
func tempConfigPathFromOutput(t *testing.T, output string) string {
	t.Helper()

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "TEMPORARY CONFIG PATH:") {
			return strings.TrimSpace(strings.Split(line, ":")[1])
		}
	}
	t.Fatalf("failed to find temporary config path in output")
	return ""
}

// sanitizesTempPaths replaces temporary config paths in the given string with a
// nondeterministic placeholder.
func sanitizeTempPaths(t *testing.T, output string) string {
	t.Helper()

	tmpPaths := regexp.MustCompile(`/tmp/[^/\n]*\.conf`)
	return tmpPaths.ReplaceAllString(output, "/tmp/aad.conf")
}
