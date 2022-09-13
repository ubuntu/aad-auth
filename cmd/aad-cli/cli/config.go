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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ubuntu/aad-auth/conf"
	"github.com/ubuntu/aad-auth/internal/config"
	"github.com/ubuntu/aad-auth/internal/consts"
	"github.com/ubuntu/aad-auth/internal/logger"
)

func (a *App) installConfig() {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage aad-auth configuration",
		Long: fmt.Sprintf(`Manage aad-auth configuration

Edit or print the configuration file at %s.`, a.options.configFile),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			edit, _ := cmd.Flags().GetBool("edit")
			domain, _ := cmd.Flags().GetString("domain")

			// Handle config printing if editing wasn't requested
			if !edit {
				return printConfig(a.ctx, a.options.configFile, domain)
			}

			// Otherwise, edit the config file
			return editConfig(a.ctx, a.options.configFile, a.options.editor)
		},
	}
	cmd.Flags().BoolP("edit", "e", false, "Edit the configuration file in an external editor")
	cmd.Flags().StringP("domain", "d", getDefaultDomain(), "Domain to use for parsing configuration")
	cmd.MarkFlagsMutuallyExclusive("edit", "domain")
	a.rootCmd.AddCommand(cmd)
}

// editConfig opens the configuration file in an external editor for editing.
func editConfig(ctx context.Context, configFile, editor string) error {
	// Create a temporary file with the previous config file contents
	tempfile, err := tempFileWithPreviousConfig(ctx, configFile)
	if err != nil {
		return err
	}

	// Run the editor on the temporary file
	//#nosec:G204 - we control the tempfile and the config path, user can override their EDITOR env var
	c := exec.Command(editor, tempfile)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to edit config: %w", err)
	}

	// Replace the current config with the temporary file if it has changed and is valid
	if err := config.Validate(ctx, tempfile); err != nil {
		return fmt.Errorf("invalid config: %w\nThe temporary file was saved at: %s", err, tempfile)
	}
	if err := os.Rename(tempfile, configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Println("The configuration at", configFile, "has been successfully updated.")
	return nil
}

// tempFileWithPreviousConfig returns a temporary file with the contents of the
// previous config file if it exists.
// If the previous config file does not exist, its contents are empty.
// If the previous config file cannot be read, an error is returned.
func tempFileWithPreviousConfig(ctx context.Context, configFile string) (string, error) {
	tempfile, err := os.Create(filepath.Join(filepath.Dir(configFile), "aad.conf.tmp"))
	if err != nil {
		return "", fmt.Errorf("failed to create temporary config file: %w", err)
	}
	defer tempfile.Close()

	config, err := os.OpenFile(configFile, os.O_RDWR, 0600)
	if err != nil {
		// If the previous config file doesn't exist, return the empty temporary file
		if errors.Is(err, fs.ErrNotExist) {
			if _, err := tempfile.Write([]byte(conf.AADConfTemplate)); err != nil {
				logger.Warn(ctx, "failed to write template to temporary config file: %v", err)
			}
			return tempfile.Name(), nil
		}
		return "", fmt.Errorf("could not open previous config file for writing: %w", err)
	}
	defer config.Close()

	if _, err := tempfile.ReadFrom(config); err != nil {
		return "", fmt.Errorf("could not read from config file: %w", err)
	}
	return tempfile.Name(), nil
}

// getDefaultDomain returns the default domain to use when parsing the config
// file, inferred from the current username.
// If no domain is found, an empty string is returned.
func getDefaultDomain() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	_, domain, _ := strings.Cut(u.Username, "@")

	return domain
}

// getDefaultEditor returns the default editor to use when editing the config file.
// It can be overridden by the user via the EDITOR env var.
func getDefaultEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return consts.DefaultEditor
}

// printConfig prints the current configuration from the passed domain in the
// ini format.
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
	fmt.Print(buf.String())

	return nil
}
