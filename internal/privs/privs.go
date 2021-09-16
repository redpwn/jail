package privs

import (
	"fmt"

	"github.com/redpwn/jail/internal/config"
	"golang.org/x/sys/unix"
)

const UserId = 1000

func DropPrivs(cfg *config.Config) error {
	if err := initSeccomp(cfg); err != nil {
		return fmt.Errorf("init seccomp: %w", err)
	}
	if err := unix.Setresgid(UserId, UserId, UserId); err != nil {
		return fmt.Errorf("setresgid jail: %w", err)
	}
	if err := unix.Setgroups([]int{UserId}); err != nil {
		return fmt.Errorf("setgroups jail: %w", err)
	}
	if err := unix.Setresuid(UserId, UserId, UserId); err != nil {
		return fmt.Errorf("setresuid jail: %w", err)
	}
	capHeader := &unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	// https://github.com/golang/go/issues/44312
	capData := [2]unix.CapUserData{}
	if err := unix.Capset(capHeader, &capData[0]); err != nil {
		return fmt.Errorf("capset: %w", err)
	}
	return nil
}
