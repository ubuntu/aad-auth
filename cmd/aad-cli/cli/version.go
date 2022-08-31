package cli

import (
	"context"
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
		Run:   func(cmd *cobra.Command, args []string) { printVersion(a.ctx, a.options.dpkgQueryCmd) },
	}
	a.rootCmd.AddCommand(cmd)
}

// printVersion prints the CLI version together with PAM/NSS information if
// applicable.
func printVersion(ctx context.Context, dpkgQueryCmd string) {
	fmt.Println("aad-cli\t\t" + consts.Version)
	printLibraryVersions(ctx, dpkgQueryCmd)
}

// printLibraryVersions queries dpkg for the PAM/NSS library versions and prints them.
// Otherwise, a "not found" message is printed.
func printLibraryVersions(ctx context.Context, dpkgQueryCmd string) {
	queryArgs := []string{"-W", "--showformat", "${Version}"}
	packages := []string{"libpam-aad", "libnss-aad"}

	for _, pkg := range packages {
		pkgQuery := append(queryArgs, pkg)

		//#nosec:G204 - process name can only be changed in tests
		c, err := exec.Command(dpkgQueryCmd, pkgQuery...).Output()
		fmt.Printf("%s\t", pkg)
		if err != nil {
			fmt.Printf("not installed\n")
			logger.Debug(ctx, "got %s error: %v", dpkgQueryCmd, err)
			continue
		}
		fmt.Printf("%s\n", c)
	}
}
