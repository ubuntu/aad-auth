package cli_test

import (
	"flag"
	"testing"

	"github.com/ubuntu/aad-auth/internal/testutils"
)

func TestMain(m *testing.M) {
	testutils.InstallUpdateFlag()
	flag.Parse()

	m.Run()
}
