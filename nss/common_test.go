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

var libPath string

// outNSSCommandForLib returns the specific part by the nss command to got, filtering originOut.
// It uses the locally build aad nss module.
func outNSSCommandForLib(t *testing.T, rootUID, rootGID, shadowMode int, cacheDir string, originOut []byte, cmds ...string) (got string, err error) {
	t.Helper()

	//! G204: Subprocess launched with a potential tainted input or cmd arguments (gosec)
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

func TestMain(m *testing.M) {
	// Build the pam module in a temporary directory and allow linking to it.
	libDir, cleanup, err := createTempDir()
	if err != nil {
		os.Exit(1)
	}

	libPath = filepath.Join(libDir, "libnss_aad.so.2")
	out, err := exec.Command("go", "build", "-buildmode=c-shared", "-tags", "integrationtests", "-o", libPath).CombinedOutput()
	if err != nil {
		cleanup()
		fmt.Fprintf(os.Stderr, "Can not build nss module (%v) : %s", err, out)
		os.Exit(1)
	}

	testutils.InstallUpdateFlag()
	flag.Parse()

	m.Run()
}

// createTempDir to create a temporary directory with a cleanup teardown not having a testing.T.
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
