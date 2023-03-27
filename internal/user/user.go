// Package user contains some helpers for user normalization.
package user

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// NormalizeName returns a normalized, lowercase version of the username as
// AnYCaSe@DomAIN is accepted by aad.
func NormalizeName(name string) string {
	return strings.ToLower(name)
}

// IsBusy returns an error if the given UID is in use by any running process.
func IsBusy(procFs string, id uint64) error {
	uid := strconv.FormatUint(id, 10)
	root, err := os.Stat("/")
	if err != nil {
		return err
	}

	statusPaths, err := filepath.Glob(fmt.Sprintf("%s/*/status", procFs))
	if err != nil {
		return err
	}

	for _, statusPath := range statusPaths {
		pid := filepath.Base(filepath.Dir(statusPath))

		// Skip processes running in a different mount namespace
		//nolint:forcetypeassert // we know it's a syscall.Stat_t
		if processRoot, err := os.Stat(fmt.Sprintf("%s/%s/root", procFs, pid)); err != nil ||
			root.Sys().(*syscall.Stat_t).Ino != processRoot.Sys().(*syscall.Stat_t).Ino ||
			root.Sys().(*syscall.Stat_t).Dev != processRoot.Sys().(*syscall.Stat_t).Dev {
			continue
		}

		// Check for the UID in the process status
		if err := checkProcessStatus(statusPath, uid, pid); err != nil {
			return err
		}

		// Check for the UID in the task statuses
		taskStatusPaths, err := filepath.Glob(fmt.Sprintf("%s/%s/task/*/status", procFs, pid))
		if err != nil {
			return err
		}

		for _, taskStatusPath := range taskStatusPaths {
			if err := checkProcessStatus(taskStatusPath, uid, pid); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkProcessStatus parses the status file of a process or task and returns an
// error if it belongs to the given UID.
func checkProcessStatus(statusPath string, uid, pid string) error {
	file, err := os.Open(statusPath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}

		// Uid: 1000 1000 1000 1000
		//      ruid euid suid fsuid
		uids := strings.Fields(line)[1:]
		for _, u := range uids {
			if u == uid {
				return fmt.Errorf("UID %s is currently used by process %s", uid, pid)
			}
		}
	}

	return nil
}
