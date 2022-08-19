package cli_test

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/cmd/aad-cli/cli"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestVersion(t *testing.T) {
	tests := map[string]struct {
		installedPkgs []string
	}{
		"both libraries installed":     {installedPkgs: []string{"libpam-aad", "libnss-aad"}},
		"only pam installed":           {installedPkgs: []string{"libpam-aad"}},
		"only nss installed":           {installedPkgs: []string{"libnss-aad"}},
		"both libraries not installed": {},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			mockCmd := newQueryMockCmd(t, tc.installedPkgs)

			c := cli.New(cli.WithDpkgQueryCmd(mockCmd))
			got, err := runApp(t, c, "version")
			require.NoError(t, err, "Version should not fail")

			want := testutils.SaveAndLoadFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected version output")
		})
	}
}

func newQueryMockCmd(t *testing.T, installedPkgs []string) string {
	t.Helper()

	tmpfile, err := os.CreateTemp(os.TempDir(), "dpkg-query.*.sh")
	require.NoError(t, err, "failed to create temporary file")
	defer tmpfile.Close()
	t.Cleanup(func() { os.Remove(tmpfile.Name()) })

	var b bytes.Buffer
	b.WriteString(`#!/bin/sh
# Loop to last argument which is the package name
echo "stderr should not be captured" >&2
for pkgname; do :; done
`)
	for _, pkg := range installedPkgs {
		b.WriteString(fmt.Sprintf(`[ "$pkgname" = "%s" ] && printf ${pkgname}-ver && exit 0`, pkg))
		b.WriteRune('\n')
	}

	b.WriteString("exit 1\n")

	err = tmpfile.Chmod(0700)
	require.NoError(t, err, "failed to set executable permissions on temporary file")

	_, err = tmpfile.Write(b.Bytes())
	require.NoError(t, err, "failed to write temporary file")

	return tmpfile.Name()
}

// runApp instantiates the CLI tool with the given args.
// It returns the stdout content and error from client.
func runApp(t *testing.T, c *cli.App, args ...string) (stdout string, err error) {
	t.Helper()

	changeAppArgs(t, c, args...)

	// capture stdout
	r, w, err := os.Pipe()
	require.NoError(t, err, "Setup: pipe shouldn’t fail")
	orig := os.Stdout
	os.Stdout = w

	err = c.Run()

	// restore and collect
	os.Stdout = orig
	w.Close()
	var out bytes.Buffer
	_, errCopy := io.Copy(&out, r)
	require.NoError(t, errCopy, "Couldn’t copy stdout to buffer")

	return out.String(), err
}

type setterArgs interface {
	SetArgs([]string)
}

// changeAppArgs modifies the application Args for cobra to parse them successfully.
// Do not share the daemon or client passed to it, as cobra store it globally.
func changeAppArgs(t *testing.T, s setterArgs, args ...string) {
	t.Helper()

	newArgs := []string{"-vv"}
	if args != nil {
		newArgs = append(newArgs, args...)
	}

	s.SetArgs(newArgs)
}

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()

	m.Run()
}
