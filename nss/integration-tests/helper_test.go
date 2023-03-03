package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/testutils"
)

// rustCovEnv defines the environment variables that need to used when running / building the rust code
// with coverage enabled.
var rustCovEnv []string
var libPath string

// outNSSCommandForLib returns the specific part for the nss command, filtering originOut.
// It uses the locally build aad nss module for the integration tests.
func outNSSCommandForLib(t *testing.T, rootUID, rootGID, shadowMode int, cacheDir string, originOut string, cmds ...string) (got string, err error) {
	t.Helper()

	// #nosec:G204 - we control the command arguments in tests
	cmd := exec.Command(cmds[0], cmds[1:]...)
	cmd.Env = append(cmd.Env, rustCovEnv...)
	cmd.Env = append(cmd.Env,
		"NSS_AAD_DEBUG=stderr",
		fmt.Sprintf("NSS_AAD_ROOT_UID=%d", rootUID),
		fmt.Sprintf("NSS_AAD_ROOT_GID=%d", rootGID),
		fmt.Sprintf("NSS_AAD_SHADOW_GID=%d", rootGID),
		fmt.Sprintf("NSS_AAD_CACHEDIR=%s", cacheDir),
		// nss needs both LD_PRELOAD and LD_LIBRARY_PATH to load the nss module lib
		fmt.Sprintf("LD_PRELOAD=%s:%s", libPath, os.Getenv("LD_PRELOAD")),
		fmt.Sprintf("LD_LIBRARY_PATH=%s:%s", filepath.Dir(libPath), os.Getenv("LD_LIBRARY_PATH")),
	)

	if shadowMode != -1 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("NSS_AAD_SHADOWMODE=%d", shadowMode))
	}

	var out bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &out)
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	got = strings.Replace(out.String(), originOut, "", 1)

	return got, err
}

// buildRustNSSLib builds the NSS library with the feature integration-tests enabled and copies the
// compiled file to libPath.
func buildRustNSSLib(t *testing.T) {
	t.Helper()

	// Gets the path to the integration-tests.
	_, p, _, _ := runtime.Caller(0)
	l := strings.Split(filepath.Dir(p), "/")
	// Walk up the tree to get the path of the project root
	aadPath := "/" + filepath.Join(l[:len(l)-2]...)

	rustDir := filepath.Join(aadPath, "nss")
	testutils.MarkRustFilesForTestCache(t, rustDir)
	var target string
	rustCovEnv, target = testutils.TrackRustCoverage(t, rustDir)

	cargo := os.Getenv("CARGO_PATH")
	if cargo == "" {
		cargo = "cargo"
	}

	// Builds the nss library.
	args := []string{"build", "--verbose", "--all-features", "--target-dir", target}
	// #nosec:G204 - we control the command arguments in tests
	cmd := exec.Command(cargo, args...)
	cmd.Env = append(os.Environ(), rustCovEnv...)
	cmd.Dir = aadPath

	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "Setup: could not build Rust NSS library: %s", out)

	// When building the crate with dh-cargo, this env is set to indicate which arquitecture the code
	// is being compiled to. When it's set, the compiled is stored under target/$(DEB_HOST_RUST_TYPE)/debug,
	// rather than under target/debug, so we need to append at the end of target to ensure we use
	// the right path.
	// If the env is not set, the target stays the same.
	target = filepath.Join(target, os.Getenv("DEB_HOST_RUST_TYPE"))

	// Creates a symlink for the compiled library with the expected versioned name.
	libPath = filepath.Join(target, "libnss_aad.so.2")
	err = os.Symlink(filepath.Join(target, "debug", "libnss_aad.so"), libPath)
	require.NoError(t, err, "Setup: failed to create versioned link to the library")
}
