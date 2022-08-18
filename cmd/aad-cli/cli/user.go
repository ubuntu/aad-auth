package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ubuntu/aad-auth/internal/cache"
)

func (a *App) installUser() {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage local Azure AD user information",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(a.installUserSet())
	cmd.AddCommand(a.installUserGet())
	a.rootCmd.AddCommand(cmd)
}

func (a *App) installUserSet() *cobra.Command {
	return &cobra.Command{
		Use:   "set <username> <key> <value>",
		Short: "Configure local Azure AD user settings",
		Args:  cobra.ExactArgs(3),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				// Get locally available users
				return a.completeWithAvailableUsers()
			case 1:
				// Get available keys
				return cache.PasswdUpdateAttributes, cobra.ShellCompDirectiveNoFileComp
			case 2:
				// Let the shell complete the value for the last argument
				return nil, cobra.ShellCompDirectiveDefault
			}

			// We already have our 2 args: no more arg completion
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cache.New(
				a.ctx,
				cache.WithCacheDir(a.options.cacheDir),
				cache.WithRootUID(a.options.rootUID), cache.WithRootGID(a.options.rootGID), cache.WithShadowGID(a.options.shadowGID),
				cache.WithShadowMode(a.options.forceShadowMode))
			if err != nil {
				return err
			}

			login, key, value := args[0], args[1], args[2]
			if err = c.UpdateUserAttribute(a.ctx, login, key, value); err != nil {
				return err
			}

			return nil
		},
	}
}

func (a *App) installUserGet() *cobra.Command {
	return &cobra.Command{
		Use:   "get [username] [key]",
		Short: "Query local Azure AD user settings",
		Args:  cobra.MaximumNArgs(2), // allow querying everything or a specific setting
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				// Get locally available users
				return a.completeWithAvailableUsers()
			case 1:
				// Get available keys
				return cache.PasswdQueryAttributes, cobra.ShellCompDirectiveNoFileComp
			}

			// We already have our 2 args: no more arg completion
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			os.Chmod("./nss/testdata/users_in_db/shadow.db", 0640)
			os.Chmod("./nss/testdata/users_in_db/passwd.db", 0644)

			var err error
			var login, key, value string

			c, err := cache.New(a.ctx,
				cache.WithCacheDir(a.options.cacheDir),
				cache.WithRootUID(a.options.rootUID), cache.WithRootGID(a.options.rootGID), cache.WithShadowGID(a.options.shadowGID),
				cache.WithShadowMode(a.options.forceShadowMode))
			if err != nil {
				return err
			}

			switch len(args) {
			case 0:
				// Return all user names if no user was specified
				var users []string
				users, err = c.GetAllUserNames(a.ctx)
				value = strings.Join(users, "\n")
			case 1:
				// Return all keys for the given user
				login = args[0]
				var user cache.UserRecord
				user, err = c.GetUserByName(a.ctx, login)
				value = fmt.Sprintf("%+v", user)
			case 2:
				// Return the value for the given key
				login = args[0]
				key = args[1]
				value, err = c.QueryUserAttribute(a.ctx, login, key)
			}

			if err != nil {
				return err
			}

			fmt.Println(value)
			return nil
		},
	}
}

func (a App) completeWithAvailableUsers() ([]string, cobra.ShellCompDirective) {
	c, err := cache.New(
		a.ctx,
		cache.WithCacheDir(a.options.cacheDir),
		cache.WithRootUID(a.options.rootUID), cache.WithRootGID(a.options.rootGID), cache.WithShadowGID(a.options.shadowGID),
		cache.WithShadowMode(a.options.forceShadowMode))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer c.Close(a.ctx)

	users, err := c.GetAllUserNames(a.ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return users, cobra.ShellCompDirectiveNoFileComp
}
