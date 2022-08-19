package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/ubuntu/aad-auth/internal/consts"
	"github.com/ubuntu/aad-auth/internal/logger"
)

func (a *App) installVersion() {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Returns the version of aad-cli and the PAM/NSS libraries if available",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { a.printVersion() },
	}
	a.rootCmd.AddCommand(cmd)
}

// printVersion prints the CLI version together with PAM/NSS information if
// applicable.
func (a App) printVersion() {
	fmt.Println("aad-cli\t\t" + consts.Version)
	a.printLibraryVersions()
}

// printLibraryVersions queries dpkg for the PAM/NSS library versions and prints them.
// Otherwise, a "not found" message is printed.
func (a App) printLibraryVersions() {
	queryArgs := []string{"-W", "--showformat", "${Version}"}
	packages := []string{"libpam-aad", "libnss-aad"}

	for _, pkg := range packages {
		pkgQuery := append(queryArgs, pkg)

		//#nosec:G204 - process name can only be changed in tests
		c, err := exec.Command(a.options.dpkgQueryCmd, pkgQuery...).Output()
		fmt.Printf("%s\t", pkg)
		if err != nil {
			fmt.Printf("not installed\n")
			logger.Debug(a.ctx, "got dpkg-query error: %v", err)
			continue
		}
		fmt.Printf("%s\n", c)
	}
}
