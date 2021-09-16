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
		return err
	}
	msg := &nsjail.NsJailConfig{}
	cfg.SetConfig(msg)
	cgroup.SetConfig(msg)
	if err := config.WriteConfig(msg); err != nil {
		return err
	}
	if err := config.MountDev(); err != nil {
		return err
	}
	if err := config.RunHook(); err != nil {
		return err
	}
	if err := server.ExecServer(cfg); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
