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
	_, proxy := cfg.NsjailListen()
	if proxy {
		return execProxy(cfg)
	}
	return execNsjail(cfg)
}
