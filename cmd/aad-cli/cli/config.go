package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ubuntu/aad-auth/internal/config"
)

func (a *App) installConfig() {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage aad-auth configuration",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(a.installConfigEdit())
	cmd.AddCommand(a.installConfigPrint())
	a.rootCmd.AddCommand(cmd)
}

func (a *App) installConfigPrint() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print",
		Short: fmt.Sprintf("Print the current configuration, parsed from %s", a.options.configFile),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printConfig(a.ctx, a.options.configFile, a.domain)
		},
	}
	cmd.Flags().StringVarP(&a.domain, "domain", "d", getDefaultDomain(), "Domain to use for parsing configuration")

	return cmd
}

func (a *App) installConfigEdit() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the configuration file in an editor",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a temporary file with the previous config file contents
			tempfile, err := tempFileWithPreviousConfig(a.options.configFile)
			if err != nil {
				return err
			}

			// Run the editor on the temporary file
			//#nosec:G204 - we control the tempfile and the config path, user can override their EDITOR env var
			c := exec.Command(a.options.editor, tempfile)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("Error: failed to edit config: %w", err)
			}

			// Replace the current config with the temporary file if it has changed and is valid
			if err := config.Validate(a.ctx, tempfile); err != nil {
				return fmt.Errorf("Error: invalid config: %w\nThe temporary file was saved at: %s", err, tempfile)
			}
			// TODO see if it's worth checking if the files are the same before doing a pointless write
			defer os.Remove(tempfile)

			newConfig, err := os.ReadFile(tempfile)
			if err != nil {
				return fmt.Errorf("Error: failed to read temporary config file: %w", err)
			}
			//#nosec:G306 these are the expected permissions for the config file
			if err := os.WriteFile(a.options.configFile, newConfig, 0640); err != nil {
				return fmt.Errorf("Error: failed to write config file: %w", err)
			}

			fmt.Println("The configuration at", a.options.configFile, "has been successfully updated.")
			return nil
		},
	}
}

// tempFileWithPreviousConfig returns a temporary file with the contents of the
// previous config file if it exists.
// If the previous config file does not exist, its contents are empty.
// If the previous config file cannot be read, an error is returned.
func tempFileWithPreviousConfig(configFile string) (string, error) {
	tempfile, err := os.CreateTemp(os.TempDir(), "aad.*.conf")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary config file: %w", err)
	}

	config, err := os.OpenFile(configFile, os.O_RDWR, 0600)
	if err != nil {
		// If the previous config file doesn't exist, return the empty temporary file
		if errors.Is(err, fs.ErrNotExist) {
			return tempfile.Name(), nil
		}
		return "", fmt.Errorf("failed to open previous config file: %w", err)
	}
	defer config.Close()

	if _, err := tempfile.ReadFrom(config); err != nil {
		return "", fmt.Errorf("could not read from config file: %w", err)
	}
	return tempfile.Name(), nil
}

func getDefaultDomain() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	_, domain, _ := strings.Cut(u.Username, "@")

	return domain
}

func getDefaultEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return "nano"
}

func printConfig(ctx context.Context, path, domain string) error {
	config, err := config.Load(ctx, path, domain)
	if err != nil {
		return err
	}

	domainSection := "default"
	if domain != "" {
		domainSection = domain
	}

	buf := new(bytes.Buffer)
	cfg, err := config.ToIni()
	if err != nil {
		return err
	}

	if _, err := cfg.WriteTo(buf); err != nil {
		return fmt.Errorf("could not write config to buffer: %w", err)
	}
	fmt.Println("[" + domainSection + "]")
	fmt.Println(buf.String())

	return nil
}
