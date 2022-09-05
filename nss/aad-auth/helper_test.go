package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ubuntu/aad-auth/internal/testutils"
)

var libPath, execPath string

// outNSSCommandForLib returns the specific part by the nss command to got, filtering originOut.
// It uses the locally build aad nss module.
func outNSSCommandForLib(t *testing.T, rootUID, rootGID, shadowMode int, cacheDir string, originOut []byte, cmds ...string) (got string, err error) {
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
	got = strings.Replace(out.String(), string(originOut), "", 1)

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

func TestMain(m *testing.M) {
	// Build the nss module in a temporary directory and allow linking to it.
	libDir, cleanup, err := createTempDir()
	if err != nil {
		os.Exit(1)
	}
	defer cleanup()

	libPath = filepath.Join(libDir, "libnss_aad.so.2")
	execPath = filepath.Join(libDir, "aad_auth")

	tmp, err := os.ReadDir("../")
	if err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Error when listing nss dir: %v", err)
		os.Exit(1)
	}

	var cFiles []string
	for _, entry := range tmp {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".c") {
			continue
		}
		cFiles = append(cFiles, entry.Name())
	}

	// Builds the nss Go cli.
	// #nosec:G204 - we control the command arguments in tests
	cmd := exec.Command("go", "build", "-tags", "integrationtests", "-o", execPath)
	err = cmd.Run()
	if err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Can not build nss Go module: %v", err)
		os.Exit(1)
	}

	// Gets the cflags
	cflags := "-g -Wall -Wextra"
	out, err := exec.Command("pkg-config", "--cflags", "glib-2.0").CombinedOutput()
	if err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Could not get the required cflags (%s): %v", out, err)
		os.Exit(1)
	}
	s := string(out)
	cflags += " " + s[:len(s)-1] // Ignoring the last \n

	// Gets the ldflags
	out, err = exec.Command("pkg-config", "--libs", "glib-2.0").CombinedOutput()
	if err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Could not get the required ldflags (%s): %v", out, err)
		os.Exit(1)
	}
	s = string(out)
	ldflags := s[:len(s)-1] // Ignoring the last \n

	// Builds the nss C library.
	command := []string{fmt.Sprintf("-DSCRIPTPATH=\"%s\"", execPath)}
	command = append(command, cFiles...)
	command = append(command, strings.Split(cflags, " ")...)
	command = append(command, strings.Split(ldflags, " ")...)
	command = append(command, "-fPIC", "-shared", "-Wl,-soname,libnss_aad.so.2", "-o", libPath)

	// #nosec:G204 - we control the command arguments in tests
	cmd = exec.Command("gcc", command...)
	cmd.Dir = ".."

	out, err = cmd.CombinedOutput()
	if err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Can not build nss library (%s): %v", out, err)
		os.Exit(1)
	}

	testutils.InstallUpdateFlag()
	flag.Parse()

	m.Run()
}
