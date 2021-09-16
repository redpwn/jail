package config

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func MountDev() error {
	if _, err := os.Stat("/srv/dev"); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := unix.Mount("/jail/dev", "/srv/dev", "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("mount dev: %w", err)
	}
	return nil
}
