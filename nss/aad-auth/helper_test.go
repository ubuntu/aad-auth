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
	// Build the nss library and executable in a temporary directory and allow linking to it.
	tmpDir, cleanup, err := createTempDir()
	if err != nil {
		os.Exit(1)
	}
	defer cleanup()

	libPath = filepath.Join(tmpDir, "libnss_aad.so.2")
	execPath = filepath.Join(tmpDir, "aad_auth")

	// Builds the nss Go cli.
	// #nosec:G204 - we control the command arguments in tests
	cmd := exec.Command("go", "build", "-tags", "integrationtests", "-o", execPath)
	if err = cmd.Run(); err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Can not build nss Go module: %v", err)
		os.Exit(1)
	}

	if err = buildNssCLib(); err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Can not build nss C library: %v", err)
		os.Exit(1)
	}

	testutils.InstallUpdateFlag()
	flag.Parse()

	m.Run()
}

func buildNssCLib() error {
	tmp, err := os.ReadDir("../")
	if err != nil {
		return fmt.Errorf("error when listing nss dir: %w", err)
	}

	// Gets the .c files required to build the nss c library.
	var cFiles []string
	for _, entry := range tmp {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".c") {
			continue
		}
		cFiles = append(cFiles, entry.Name())
	}

	// Gets the cflags.
	cflags := "-g -Wall -Wextra"
	out, err := exec.Command("pkg-config", "--cflags", "glib-2.0").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not get the required cflags (%s): %w", out, err)
	}
	s := string(out)
	cflags += " " + s[:len(s)-1] // Ignoring the last \n.

	// Gets the ldflags
	out, err = exec.Command("pkg-config", "--libs", "glib-2.0").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not get the required ldflags (%s): %w", out, err)
	}
	s = string(out)
	ldflags := s[:len(s)-1] // Ignoring the last \n.

	// Assembles the flags required to build the nss library.
	c := []string{fmt.Sprintf(`-DSCRIPTPATH="%s"`, execPath)}
	c = append(c, cFiles...)
	c = append(c, strings.Split(cflags, " ")...)
	c = append(c, strings.Split(ldflags, " ")...)
	c = append(c, "-fPIC", "-shared", "-Wl,-soname,libnss_aad.so.2", "-o", libPath)

	// #nosec:G204 - we control the command arguments in tests.
	cmd := exec.Command("gcc", c...)
	cmd.Dir = ".."
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("can not build nss library (%s): %w", out, err)
	}

	return nil
}
