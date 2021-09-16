package server

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/redpwn/jail/internal/config"
	"github.com/redpwn/jail/internal/privs"
	"golang.org/x/sys/unix"
)

const nsjailPath = "/jail/nsjail"

func runNsjailChild(errCh chan<- error) {
	cmd := exec.Command(nsjailPath, "-C", config.NsjailConfigPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		errCh <- fmt.Errorf("run nsjail child: %w", err)
	}
}

func execNsjail(cfg *config.Config) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := privs.DropPrivs(cfg); err != nil {
		return err
	}
	if err := unix.Exec(nsjailPath, []string{"nsjail", "-C", config.NsjailConfigPath}, os.Environ()); err != nil {
		return fmt.Errorf("exec nsjail: %w", err)
	}
	return nil
}
