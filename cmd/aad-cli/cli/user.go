package cli

import (
	"context"
	"fmt"
	"os/user"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/logger"
)

func (a *App) installUser() {
	cmd := &cobra.Command{
		Use:   "user [key] [value]",
		Short: "Manage local Azure AD user information",
		Long: fmt.Sprintf(`Manage local Azure AD user information

When called without arguments, this command will retrieve the cache record for the current user.

Specific values can be retrieved by passing an attribute name.
Values can be set by passing an attribute name and a value.

Currently the only modifiable attributes are: %s.`, strings.Join(cache.PasswdUpdateAttributes, ", ")),
		Args: cobra.MaximumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				// Get available keys
				return cache.PasswdQueryAttributes, cobra.ShellCompDirectiveNoFileComp
			case 1:
				// Let the shell complete the value for the last argument
				return nil, cobra.ShellCompDirectiveDefault
			}

			// We already have our 2 args: no more arg completion
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := a.getCache()
			if err != nil {
				return err
			}

			username, _ := cmd.Flags().GetString("name")
			allUsers, _ := cmd.Flags().GetBool("all")

			return runUser(a.ctx, args, c, username, allUsers)
		},
	}
	cmd.Flags().StringP("name", "n", getDefaultUser(), "username to operate on")
	cmd.Flags().BoolP("all", "a", false, "list all users")
	cmd.MarkFlagsMutuallyExclusive("name", "all")

	// Register completion for the --name flag
	if err := cmd.RegisterFlagCompletionFunc("name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return a.completeWithAvailableUsers()
	}); err != nil {
		logger.Warn(a.ctx, "Unable to register completion for user command: %v", err)
	}

	a.rootCmd.AddCommand(cmd)
}

// completeWithAvailableUsers returns a list of users available in the local cache.
func (a App) completeWithAvailableUsers() ([]string, cobra.ShellCompDirective) {
	c, err := a.getCache()
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

// getCache returns the cache, either from the options field if overridden or
// a newly created one.
func (a *App) getCache() (*cache.Cache, error) {
	if a.options.cache != nil {
		return a.options.cache, nil
	}

	return cache.New(a.ctx)
}

// getDefaultUser returns the current user name or a blank string if an error occurs.
func getDefaultUser() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}

	return u.Username
}

// runUser executes a specific user action based on the arguments passed to the command.
func runUser(ctx context.Context, args []string, c *cache.Cache, username string, allUsers bool) error {
	var err error
	var key string
	var value any

	switch len(args) {
	case 0:
		// Return current user information or all user names if explicitly requested.
		if allUsers {
			var users []string
			users, err = c.GetAllUserNames(ctx)
			value = strings.Join(users, "\n")
		} else {
			var user cache.UserRecord
			user, err = c.GetUserByName(ctx, username)
			value, _ = user.IniString()
		}
	case 1:
		// Return the value for the given key
		key = args[0]
		if key == "shadow_password" {
			if !c.ShadowReadable() {
				return fmt.Errorf("You do not have permission to read the shadow database")
			}
			var user cache.UserRecord
			user, err = c.GetUserByName(ctx, username)
			value = user.ShadowPasswd
			break
		}

		value, err = c.QueryPasswdAttribute(ctx, username, key)
	case 2:
		// Set the value for the given key and exit
		key, value = args[0], args[1]
		if err := c.UpdateUserAttribute(ctx, username, key, value); err != nil {
			return err
		}
		return nil
	}

	if err != nil {
		return err
	}

	fmt.Println(strings.TrimSpace(fmt.Sprint(value)))
	return nil
}
