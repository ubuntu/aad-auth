package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	osuser "os/user"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/i18n"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/user"
	"github.com/ubuntu/decorate"
	"golang.org/x/exp/slices"
	"golang.org/x/sys/unix"
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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("move-home") && (len(args) < 2 || !slices.Contains(args, "home")) {
				return fmt.Errorf("move-home can only be used when modifying home attribute")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := a.getCache()
			if err != nil {
				return err
			}

			username, _ := cmd.Flags().GetString("name")
			allUsers, _ := cmd.Flags().GetBool("all")
			moveHome, _ := cmd.Flags().GetBool("move-home")

			return runUser(a.ctx, args, c, a.options.procFs, username, allUsers, moveHome)
		},
	}
	cmd.Flags().StringP("name", "n", a.options.currentUser, "username to operate on")
	cmd.Flags().BoolP("all", "a", false, "list all users")
	cmd.Flags().BoolP("move-home", "m", false, "if updating home, move the content of the home directory to the new location")
	cmd.MarkFlagsMutuallyExclusive("name", "all")
	cmd.MarkFlagsMutuallyExclusive("move-home", "all")

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
	u, err := osuser.Current()
	if err != nil {
		return ""
	}

	return u.Username
}

// runUser executes a specific user action based on the arguments passed to the command.
func runUser(ctx context.Context, args []string, c *cache.Cache, procFs, username string, allUsers bool, moveHome bool) error {
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
		if key == "last_online_auth" {
			i, ok := value.(int64)
			if !ok {
				err = fmt.Errorf("failed to parse last_online_auth as the value isn't valid: %w", err)
				break
			}
			value = time.Unix(i, 0).Format(time.RFC3339)
		}

	case 2:
		// Set the value for the given key and exit
		key, value = args[0], args[1]
		if err := updateUserAttribute(ctx, c, procFs, username, key, value, moveHome); err != nil {
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

// updateUserAttribute updates the given attribute for an user to the specified value.
// For some attributes such as home, additional actions are performed.
func updateUserAttribute(ctx context.Context, c *cache.Cache, procFs, username, key string, value any, moveHome bool) (err error) {
	defer decorate.OnError(&err, i18n.G("couldn't update attribute"))

	prevValue, err := c.QueryPasswdAttribute(ctx, username, key)
	if err != nil {
		return err
	}

	if prevValue == value {
		logger.Debug(ctx, "No change to %q for %s", key, username)
		return nil
	}

	// Don't change the attribute if moving home was requested and the user is logged in.
	if moveHome {
		uid, err := c.QueryPasswdAttribute(ctx, username, "uid")
		if err != nil {
			return err
		}

		id, ok := uid.(int64)
		if !ok {
			return fmt.Errorf("invalid uid type: %T", uid)
		}
		if err := user.IsBusy(procFs, uint64(id)); err != nil {
			return err
		}
	}

	if err := c.UpdateUserAttribute(ctx, username, key, value); err != nil {
		return err
	}

	// Take additional actions based on the key that was updated
	if key == "home" && moveHome {
		// Update the home directory if it changed
		if err := moveUserHome(ctx, username, fmt.Sprintf("%v", prevValue), fmt.Sprintf("%v", value)); err != nil {
			return err
		}
	}
	return nil
}

// moveUserHome moves the home directory of an user from the previous location to the new one.
func moveUserHome(ctx context.Context, username, prevValue, value string) (err error) {
	defer decorate.OnError(&err, i18n.G("unable to move home directory for %s"), username)

	// Does the target directory exist?
	if unix.Access(value, unix.F_OK) == nil {
		return fmt.Errorf("directory %q already exists", value)
	}

	homeInfo, err := os.Stat(prevValue)
	if err != nil {
		return err
	}
	if !homeInfo.IsDir() {
		return fmt.Errorf("%q was not a directory, it is not removed and no home directories are created", prevValue)
	}

	// Try renaming first
	if err := os.Rename(prevValue, value); err == nil {
		logger.Debug(ctx, "Moved %q to %q", prevValue, value)
		return nil
	}

	// Try renaming with mv to account for cross-device links
	logger.Debug(ctx, "Unable to rename %q to %q. Trying with mv", prevValue, value)
	if err := exec.Command("mv", prevValue, value).Run(); err != nil {
		return err
	}

	logger.Debug(ctx, "Moved home directory for %s from %q to %q", username, prevValue, value)
	return nil
}
