// Package cli contains the CLI implementation.
package cli

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/consts"
	"github.com/ubuntu/aad-auth/internal/logger"
)

// App encapsulates commands and options of the application.
type App struct {
	rootCmd cobra.Command
	ctx     context.Context

	options options
}

// options are the configurable functional options of the application.
type options struct {
	editor       string
	configFile   string
	dpkgQueryCmd string
	procFs       string
	currentUser  string
	cache        *cache.Cache
}
type option func(*options)

// New registers commands and returns a new App.
func New(opts ...option) *App {
	// Apply given options.
	args := options{
		editor:       getDefaultEditor(),
		configFile:   consts.DefaultConfigPath,
		dpkgQueryCmd: "dpkg-query",
		currentUser:  getDefaultUser(),
		procFs:       "/proc",
	}

	for _, o := range opts {
		o(&args)
	}

	a := App{ctx: context.Background(), options: args}
	a.rootCmd = cobra.Command{
		Use:   "aad-cli [COMMAND]",
		Short: "Azure AD CLI",
		Long:  "Manage Azure AD accounts configuration",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			// Set logger parameters and attach to the context
			verbosity, _ := cmd.Flags().GetCount("verbose")
			logger.SetVerboseMode(verbosity)
			logrus.SetFormatter(&logger.LogrusFormatter{})
			a.ctx = logger.CtxWithLogger(a.ctx, logger.LogrusLogger{FieldLogger: logrus.StandardLogger()})

			return nil
		},
	}

	a.rootCmd.PersistentFlags().CountP("verbose", "v", "issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output")

	a.installUser()
	a.installConfig()
	a.installVersion()

	return &a
}

// Run executes the app.
func (a *App) Run() error {
	return a.rootCmd.Execute()
}

// Quit exits the app.
func (a *App) Quit() error {
	return nil
}

// UsageError returns if the error is a command parsing or runtime one.
func (a App) UsageError() bool {
	return !a.rootCmd.SilenceUsage
}

// SetArgs changes the root command args. Shouldn't be in general necessary apart for integration tests.
func (a *App) SetArgs(args []string) {
	a.rootCmd.SetArgs(args)
}

// RootCmd returns a copy of the root command for the app. Shouldn't be in
// general necessary apart from running generators.
func (a App) RootCmd() cobra.Command {
	return a.rootCmd
}
