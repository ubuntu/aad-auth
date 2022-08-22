package testutils

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/cmd/aad-cli/cli"
)

// RunApp instantiates the CLI tool with the given args.
// It returns the stdout content and error from client.
func RunApp(t *testing.T, c *cli.App, args ...string) (stdout string, err error) {
	t.Helper()

	ChangeAppArgs(t, c, args...)

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

// ChangeAppArgs modifies the application Args for cobra to parse them successfully.
// Do not share the daemon or client passed to it, as cobra store it globally.
func ChangeAppArgs(t *testing.T, s setterArgs, args ...string) {
	t.Helper()

	newArgs := []string{"-vv"}
	if args != nil {
		newArgs = append(newArgs, args...)
	}

	s.SetArgs(newArgs)
}
