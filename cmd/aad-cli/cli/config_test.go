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
		"homedir and shell optional fields missing":       {configFile: "missing-homedir-and-shell-fields.conf"},
		"required entries only present in default domain": {domain: "example.com", configFile: "required-present-in-default-domain.conf"},

		// error cases
		"missing required entries": {configFile: "missing-required.conf", wantErr: true},
		"non-existent config":      {configFile: "non-existent.conf", wantErr: true},
		"malformed config":         {configFile: "malformed.conf", wantErr: true},
		"type mismatch":            {configFile: "type-mismatch.conf", wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			cmdArgs := []string{"config"}

			if tc.domain != "" {
				cmdArgs = append(cmdArgs, "--domain", tc.domain)
			}

			if tc.configFile == "" {
				tc.configFile = "aad.conf"
			}
			tc.configFile = filepath.Join("testdata", tc.configFile)

			c := cli.New(cli.WithConfigFile(tc.configFile))
			got, err := testutils.RunApp(t, c, cmdArgs...)

			if tc.wantErr {
				require.Error(t, err, "expected command to return an error")
				return
			}
			require.NoError(t, err, "expected command to succeed")

			want := testutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestConfigEdit(t *testing.T) {
	requiredConfig := "tenant_id = something\napp_id = something"
	badConfig := "tenant_id = something"
	malformedConfig := "aaaaaaaaaaaaa"

	tests := map[string]struct {
		configFile       string
		newConfigContent string

		wantErr       bool
		wantEditorErr bool
	}{
		"loads the previous config": {},

		// This test asserts that the config template is loaded when executing the command with an absent config file
		// (see TEMPORARY CONFIG CONTENTS in the editor mock).
		// To avoid getting an error on save, we have to pass in a valid config
		// since the template is commented out, hence the need for newConfigContent.
		"loads the config template if previous is not present": {configFile: "nonexistent.conf", newConfigContent: requiredConfig},

		// error cases
		"editor returns an error":         {wantEditorErr: true, wantErr: true},
		"cfg validation returns an error": {newConfigContent: badConfig, wantErr: true},
		"cfg loading returns an error":    {newConfigContent: malformedConfig, wantErr: true},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.configFile == "" {
				tc.configFile = "aad.conf"
			}
			tc.configFile = filepath.Join("testdata", tc.configFile)

			// Copy the config file to a temporary location, so that changes do
			// not persist across tests.
			tc.configFile = copyToTempPath(t, tc.configFile)
			editorMock := newEditorMock(t, tc.configFile, tc.newConfigContent, tc.wantEditorErr)

			c := cli.New(cli.WithConfigFile(tc.configFile), cli.WithEditor(editorMock))
			got, err := testutils.RunApp(t, c, "config", "-e")

			tempConfigPath := tempConfigPathFromOutput(t, got)
			if tc.wantErr {
				require.Error(t, err, "expected command to return an error")
				require.FileExists(t, tempConfigPath, "expected temporary config file to be present")
				return
			}
			require.NoError(t, err, "expected command to succeed")
			require.NoFileExists(t, tempConfigPath, "expected temporary config file not to be present")

			got = sanitizeTempPaths(t, got)
			want := testutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "expected output to match golden file")
		})
	}
}

func TestConfigEditor(t *testing.T) {
	// Custom editor
	err := os.Setenv("EDITOR", "vim")
	require.NoError(t, err, "Setup: failed to set EDITOR")

	c := cli.New()
	require.Equal(t, "vim", c.Editor(), "expected editor to be vim")

	// Default behavior
	err = os.Unsetenv("EDITOR")
	require.NoError(t, err, "Setup: failed to unset EDITOR")

	c = cli.New()
	require.Equal(t, "sensible-editor", c.Editor(), "expected default editor to be sensible-editor")
}

func TestConfigMutuallyExclusiveFlags(t *testing.T) {
	c := cli.New()
	_, err := testutils.RunApp(t, c, "config", "--edit", "--domain", "example.com")
	require.ErrorContains(t, err, "if any flags in the group [edit domain] are set none of the others can be", "expected command to return mutually exclusive flag error")

	// Short flags
	_, err = testutils.RunApp(t, c, "config", "-e", "-d", "example.com")
	require.ErrorContains(t, err, "if any flags in the group [edit domain] are set none of the others can be", "expected command to return mutually exclusive flag error")
}

// newEditorMock returns the path to a shell script that overrides the default
// config editor.
// The script prints the previous config file, and if a new config is provided,
// it replaces the previous config file with the new contents.
func newEditorMock(t *testing.T, configFile, newConfig string, wantErr bool) string {
	t.Helper()

	editor, err := os.CreateTemp(t.TempDir(), "editor-mock.*.sh")
	require.NoError(t, err, "Setup: failed to create temporary file")
	defer editor.Close()

	var b bytes.Buffer
	b.WriteString("#!/bin/sh")

	b.WriteString(`
echo "TEMPORARY CONFIG PATH: $1"
echo "TEMPORARY CONFIG CONTENTS:"
cat $1`)
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
	require.NoError(t, err, "Setup: failed to set executable permissions on temporary file")

	_, err = editor.Write(b.Bytes())
	require.NoError(t, err, "Setup: failed to write temporary file")

	return editor.Name()
}

// copyToTempPath copies the given config file to a temporary location, and returns the path to it.
// If the given config file is not present, a non-existent temporary path is returned.
func copyToTempPath(t *testing.T, file string) string {
	t.Helper()

	tempdir := t.TempDir()
	r, err := os.Open(file)
	if err != nil {
		// We assume a non-existent file was a deliberate choice,
		// so return back a non-existent file in a temporary directory.
		return filepath.Join(tempdir, filepath.Base(file))
	}
	defer r.Close()

	w, err := os.Create(filepath.Join(tempdir, "aad.conf"))
	require.NoError(t, err, "Setup: failed to create temporary file")
	defer w.Close()

	_, err = w.ReadFrom(r)
	require.NoError(t, err, "Setup: failed to copy file")

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
// deterministic placeholder.
func sanitizeTempPaths(t *testing.T, output string) string {
	t.Helper()

	tmpPaths := regexp.MustCompile(`/tmp/.*\.conf[^\s]*`)
	return tmpPaths.ReplaceAllString(output, "/tmp/aad.conf")
}
