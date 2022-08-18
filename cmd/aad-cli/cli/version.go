package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/ubuntu/aad-auth/internal/consts"
)

func (a *App) installVersion() {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Returns version of aad-cli and exits",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { printVersion() },
	}
	a.rootCmd.AddCommand(cmd)
}

// printVersion prints the CLI version together with PAM/NSS information if
// applicable.
func printVersion() {
	fmt.Println("aad-cli\t" + consts.Version)
	getLibraryVersions()
}

func getLibraryVersions() {
	queryArgs := []string{"-W", "--showformat", "${Package} ${Version}\n"}
	queryArgs = append(queryArgs, "libpam-aad", "libnss-aad")

	c, err := exec.Command("dpkg-query", queryArgs...).Output()
	if len(c) == 0 {
		fmt.Println("The AAD PAM and NSS libraries are not installed.")
		return
	}
	if err != nil && len(c) > 0 {
		fmt.Println("One or more libraries are not installed:")
	}
	fmt.Println(string(c))
}
