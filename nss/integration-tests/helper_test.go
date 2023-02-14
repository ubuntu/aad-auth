package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var targetDir, libPath string

// outNSSCommandForLib returns the specific part for the nss command, filtering originOut.
// It uses the locally build aad nss module for the integration tests.
func outNSSCommandForLib(t *testing.T, rootUID, rootGID, shadowMode int, cacheDir string, originOut string, cmds ...string) (got string, err error) {
	t.Helper()

	// #nosec:G204 - we control the command arguments in tests
	cmd := exec.Command(cmds[0], cmds[1:]...)
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

// createTempDir creates a temporary directory with a cleanup teardown not having a testing.T.
func createTempDir() (tmp string, cleanup func(), err error) {
	if tmp, err = os.MkdirTemp("", "aad-auth-integration-tests-nss"); err != nil {
		fmt.Fprintf(os.Stderr, "Can not create temporary directory %q", tmp)
		return "", nil, err
	}
	return tmp, func() {
		if err := os.RemoveAll(tmp); err != nil {
			fmt.Fprintf(os.Stderr, "Can not clean up temporary directory %q", tmp)
		}
	}, nil
}

// buildRustNSSLib builds the NSS library with the feature integration-tests enabled and copies the
// compiled file to libPath.
func buildRustNSSLib() error {
	aadPath, err := filepath.Abs("../..")
	if err != nil {
		return err
	}
	// Builds the nss library.
	args := []string{"build", "--verbose", "--features", "integration-tests", "--target-dir", targetDir}

	cargo := os.Getenv("CARGO_PATH")
	if cargo == "" {
		cargo = "cargo"
	}
	// #nosec:G204 - we control the command arguments in tests
	cmd := exec.Command(cargo, args...)
	cmd.Dir = aadPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not build rust nss library (%s): %w", out, err)
	}
	targetDir = filepath.Join(targetDir, os.Getenv("DEB_HOST_RUST_TYPE"))

	// Renames the compiled library to have the expected versioned name.
	if err = os.Rename(filepath.Join(targetDir, "debug", "libnss_aad.so"), libPath); err != nil {
		return fmt.Errorf("Setup: could not rename the Rust NSS library: %w", err)
	}
	return nil
}
