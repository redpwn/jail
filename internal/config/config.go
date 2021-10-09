package config

//go:generate mkdir -p ../proto
//go:generate protoc -I../../nsjail --go_out ../proto --go_opt Mconfig.proto=/nsjail config.proto

import (
	"fmt"
	"os"

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
	Syscalls   []string `env:"JAIL_SYSCALLS"`
	TmpSize    size     `env:"JAIL_TMP_SIZE"`
}

const NsjailConfigPath = "/tmp/nsjail.cfg"

func (c *Config) SetConfig(msg *nsjail.NsJailConfig) {
	msg.Mode = nsjail.Mode_LISTEN.Enum()
	msg.TimeLimit = &c.Time
	msg.RlimitAsType = nsjail.RLimit_HARD.Enum()
	msg.RlimitCpuType = nsjail.RLimit_HARD.Enum()
	msg.RlimitFsizeType = nsjail.RLimit_HARD.Enum()
	msg.RlimitNofileType = nsjail.RLimit_HARD.Enum()
	msg.CgroupPidsMax = &c.Pids
	msg.CgroupMemMax = proto.Uint64(uint64(c.Mem))
	msg.CgroupCpuMsPerSec = &c.Cpu
	// kafel umount is umount2
	// https://github.com/google/kafel/blob/f67ddf5acf57fb7de1e25500cc266c1588ecf3f1/src/syscalls/amd64_syscalls.c#L2041-L2046
	msg.SeccompString = []string{`
		ERRNO(1) {
			clone { (clone_flags & 0x7e020000) != 0 },
			mount, sethostname, umount, pivot_root
		}
		DEFAULT ALLOW
	`}
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
	if c.Pow > 0 {
		msg.Bindhost = proto.String("127.0.0.1")
		msg.Port = proto.Uint32(c.Port + 1)
	} else {
		msg.Port = &c.Port
		msg.MaxConns = &c.Conns
		msg.MaxConnsPerIp = &c.ConnsPerIp
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
	return cfg, nil
}
