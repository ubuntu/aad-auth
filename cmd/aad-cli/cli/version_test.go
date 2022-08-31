package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/cmd/aad-cli/cli"
	"github.com/ubuntu/aad-auth/internal/consts"
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
			got, err := testutils.RunApp(t, c, "version")
			require.NoError(t, err, "Version should not fail")
			got = sanitizeDevVersion(got)

			want := testutils.LoadWithUpdateFromGolden(t, got)
			require.Equal(t, want, got, "Should get expected version output")
		})
	}
}

func newQueryMockCmd(t *testing.T, installedPkgs []string) string {
	t.Helper()

	tmpfile, err := os.Create(filepath.Join(t.TempDir(), "dpkg-query.sh"))
	require.NoError(t, err, "Setup: failed to create temporary file")
	defer tmpfile.Close()

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
	require.NoError(t, err, "Setup: failed to set executable permissions on temporary file")

	_, err = tmpfile.Write(b.Bytes())
	require.NoError(t, err, "Setup: failed to write temporary file")

	return tmpfile.Name()
}

func sanitizeDevVersion(s string) string {
	return strings.ReplaceAll(s, consts.Version, "dev")
}
