package main

import (
	"fmt"
	"os"

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
	cgroup, err := cgroup.ReadCgroup()
	if err != nil {
		return err
	}
	if err := cgroup.Mount(); err != nil {
		return fmt.Errorf("delegate cgroup: %w", err)
	}
	msg := &nsjail.NsJailConfig{}
	cfg.SetConfig(msg)
	cgroup.SetConfig(msg)
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
