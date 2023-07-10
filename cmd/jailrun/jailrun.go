package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/redpwn/jail/internal/cgroup"
	"github.com/redpwn/jail/internal/config"
	"github.com/redpwn/jail/internal/proto/nsjail"
	"github.com/redpwn/jail/internal/server"
)

func run() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}
	if len(os.Args) > 1 && os.Args[1] == "proxy" {
		return server.RunProxy(cfg)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := cgroup.Unshare(); err != nil {
		return fmt.Errorf("%w (is the container not privileged?)", err)
	}
	cg, err := cgroup.ReadCgroup()
	if err != nil {
		return err
	}
	if err := cg.Mount(); err != nil {
		return fmt.Errorf("delegate cgroup: %w", err)
	}
	msg := &nsjail.NsJailConfig{}
	if err := cfg.SetConfig(msg); err != nil {
		return err
	}
	if err := cg.SetConfig(msg); err != nil {
		return err
	}
	if err := config.WriteConfig(msg); err != nil {
		return err
	}
	if err := config.MountDev(cfg.Dev); err != nil {
		return err
	}
	if err := config.RunHook(); err != nil {
		return err
	}
	return server.ExecServer(cfg)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
