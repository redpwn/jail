package server

import (
	"github.com/redpwn/jail/internal/config"
)

func RunProxy(cfg *config.Config) error {
	errCh := make(chan error)
	go runNsjailChild(errCh)
	go startProxy(cfg, errCh)
	return <-errCh
}

func ExecServer(cfg *config.Config) error {
	if cfg.Pow > 0 {
		if err := execProxy(cfg); err != nil {
			return err
		}
	} else {
		if err := execNsjail(cfg); err != nil {
			return err
		}
	}
	return nil
}
