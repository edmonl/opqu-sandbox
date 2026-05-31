package util

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

type User struct {
	*user.User
	UID int
	GID int
}

func newUser(u *user.User) (*User, error) {
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID %v for %v: %w", u.Uid, u.Username, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID %v for %v: %w", u.Gid, u.Username, err)
	}

	return &User{
		User: u,
		UID:  uid,
		GID:  gid,
	}, nil
}

// InvokingUser returns the user that launched sbx, using SUDO_USER after sudo escalation.
func InvokingUser() (*User, error) {
	if os.Geteuid() == 0 {
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			u, err := user.Lookup(sudoUser)
			if err != nil {
				return nil, fmt.Errorf("failed to look up invoking user %v: %w", sudoUser, err)
			}
			return newUser(u)
		}
	}

	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	return newUser(u)
}

func ChownToUser(path string, u *User) error {
	if err := os.Chown(path, u.UID, u.GID); err != nil {
		return fmt.Errorf("failed to change ownership of %v to %v: %w", path, u.Username, err)
	}
	return nil
}
