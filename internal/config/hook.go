package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const hookPath = "/jail/hook.sh"

func RunHook() error {
	if _, err := os.Stat(hookPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(
		os.Environ(),
		"nsjail_cfg="+NsjailConfigPath,
		// TODO: add cgroup info
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec hook: %w", err)
	}
	return nil
}
