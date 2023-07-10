package config

//go:generate mkdir -p ../proto
//go:generate protoc -I../../nsjail --go_out ../proto --go_opt Mconfig.proto=/nsjail config.proto

import (
	"fmt"
	"os"
	"strings"
	"errors"

	"github.com/caarlos0/env/v6"
	"github.com/docker/go-units"
	"github.com/redpwn/jail/internal/proto/nsjail"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type size uint64

func (s *size) UnmarshalText(t []byte) error {
	v, err := units.RAMInBytes(string(t))
	*s = size(v)
	return err
}

type Config struct {
	Time       uint32   `env:"JAIL_TIME" envDefault:"20"`
	Conns      uint32   `env:"JAIL_CONNS"`
	ConnsPerIp uint32   `env:"JAIL_CONNS_PER_IP"`
	Pids       uint64   `env:"JAIL_PIDS" envDefault:"5"`
	Mem        size     `env:"JAIL_MEM" envDefault:"5M"`
	Cpu        uint32   `env:"JAIL_CPU" envDefault:"100"`
	Pow        uint32   `env:"JAIL_POW"`
	Port       uint32   `env:"JAIL_PORT" envDefault:"5000"`
	Dev        []string `env:"JAIL_DEV" envDefault:"null,zero,urandom"`
	Syscalls   []string `env:"JAIL_SYSCALLS"`
	TmpSize    size     `env:"JAIL_TMP_SIZE"`
	Env        []string
}

const envPrefix = "JAIL_ENV_"

func (c *Config) NsjailListen() (uint32, bool) {
	if c.Pow <= 0 {
		return c.Port, false
	}
	return c.Port + 1, true
}

const NsjailConfigPath = "/tmp/nsjail.cfg"

func checkExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (c *Config) SetConfig(msg *nsjail.NsJailConfig) error {
	msg.Mode = nsjail.Mode_LISTEN.Enum()
	msg.TimeLimit = &c.Time
	msg.RlimitAsType = nsjail.RLimit_HARD.Enum()
	msg.RlimitCpuType = nsjail.RLimit_HARD.Enum()
	msg.RlimitFsizeType = nsjail.RLimit_HARD.Enum()
	msg.RlimitNofileType = nsjail.RLimit_HARD.Enum()
	msg.CgroupPidsMax = &c.Pids
	msg.CgroupMemMax = proto.Uint64(uint64(c.Mem))
	msg.CgroupCpuMsPerSec = &c.Cpu
	msg.Mount = []*nsjail.MountPt{{
		Src:    proto.String("/srv"),
		Dst:    proto.String("/"),
		IsBind: proto.Bool(true),
		Nodev:  proto.Bool(true),
		Nosuid: proto.Bool(true),
	}}
	msg.Hostname = proto.String("app")
	msg.Cwd = proto.String("/app")
	msg.ExecBin = &nsjail.Exe{
		Path: proto.String("/app/run"),
	}
	port, willProxy := c.NsjailListen()
	msg.Port = &port
	if willProxy {
		msg.Bindhost = proto.String("127.0.0.1")
	} else {
		msg.MaxConns = &c.Conns
		msg.MaxConnsPerIp = &c.ConnsPerIp
	}
	proc, err := checkExists("/srv/proc")
	if err != nil {
		return err
	}
	if proc {
		msg.Mount = append(msg.Mount, &nsjail.MountPt{
			Dst:    proto.String("/proc"),
			Fstype: proto.String("proc"),
			Nodev:  proto.Bool(true),
			Nosuid: proto.Bool(true),
			Noexec: proto.Bool(true),
		})
	}
	if c.TmpSize > 0 {
		msg.Mount = append(msg.Mount, &nsjail.MountPt{
			Dst:     proto.String("/tmp"),
			Fstype:  proto.String("tmpfs"),
			Rw:      proto.Bool(true),
			Options: proto.String(fmt.Sprintf("size=%d", c.TmpSize)),
			Nodev:   proto.Bool(true),
			Nosuid:  proto.Bool(true),
		})
	}
	msg.Envar = c.Env
	return nil
}

const tmpMountFlags = uintptr(unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_RELATIME)

func mountTmp() error {
	if err := unix.Mount("", "/tmp", "tmpfs", tmpMountFlags, ""); err != nil {
		return fmt.Errorf("mount tmpfs: %w", err)
	}
	return nil
}

func WriteConfig(msg *nsjail.NsJailConfig) error {
	if err := mountTmp(); err != nil {
		return err
	}
	content, err := prototext.Marshal(msg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(NsjailConfigPath, content, 0644); err != nil {
		return err
	}
	return nil
}

func GetConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env config: %w", err)
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, envPrefix) {
			cfg.Env = append(cfg.Env, strings.TrimPrefix(e, envPrefix))
		}
	}
	return cfg, nil
}
