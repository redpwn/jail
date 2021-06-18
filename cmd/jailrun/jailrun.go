package main

//go:generate mkdir -p ../../proto
//go:generate protoc -I../../nsjail --go_out ../../proto --go_opt Mconfig.proto=/nsjail config.proto

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/caarlos0/env/v6"
	"github.com/docker/go-units"
	"github.com/redpwn/jail/proto/nsjail"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

const (
	cgroupPath    = "/jail/cgroup"
	nsjailCfgPath = "/tmp/nsjail.cfg"
	hookPath      = "/jail/hook.sh"
	mountFlags    = uintptr(unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_RELATIME)
	nsjailId      = 1000
)

type size uint64

func (s *size) UnmarshalText(t []byte) error {
	v, err := units.RAMInBytes(string(t))
	*s = size(v)
	return err
}

type jailConfig struct {
	Time       uint32   `env:"JAIL_TIME" envDefault:"20"`
	Conns      uint32   `env:"JAIL_CONNS"`
	ConnsPerIp uint32   `env:"JAIL_CONNS_PER_IP"`
	Pids       uint64   `env:"JAIL_PIDS" envDefault:"5"`
	Mem        size     `env:"JAIL_MEM" envDefault:"5M"`
	Cpu        uint32   `env:"JAIL_CPU" envDefault:"100"`
	Pow        uint32   `env:"JAIL_POW"`
	Port       uint32   `env:"JAIL_PORT" envDefault:"5000"`
	Syscalls   []string `env:"JAIL_SYSCALLS"`
	ReadOnly   bool     `env:"JAIL_READ_ONLY" envDefault:"true"`
	cgroup     *cgroupInfo
}

func writeConfig(cfg *jailConfig) error {
	msg := &nsjail.NsJailConfig{
		Mode:              nsjail.Mode_LISTEN.Enum(),
		Port:              &cfg.Port,
		TimeLimit:         &cfg.Time,
		MaxConns:          &cfg.Conns,
		MaxConnsPerIp:     &cfg.ConnsPerIp,
		RlimitAsType:      nsjail.RLimit_HARD.Enum(),
		RlimitCpuType:     nsjail.RLimit_HARD.Enum(),
		RlimitFsizeType:   nsjail.RLimit_HARD.Enum(),
		RlimitNofileType:  nsjail.RLimit_HARD.Enum(),
		CgroupPidsMax:     &cfg.Pids,
		CgroupMemMax:      proto.Uint64(uint64(cfg.Mem)),
		CgroupCpuMsPerSec: &cfg.Cpu,
		// kafel umount is umount2
		// https://github.com/google/kafel/blob/f67ddf5acf57fb7de1e25500cc266c1588ecf3f1/src/syscalls/amd64_syscalls.c#L2041-L2046
		SeccompString: []string{`
			ERRNO(1) {
				clone { (clone_flags & 0x7e020000) != 0 },
				mount, sethostname, umount, pivot_root
			}
			DEFAULT ALLOW
		`},
		Mount: []*nsjail.MountPt{{
			Src:    proto.String("/srv"),
			Dst:    proto.String("/"),
			IsBind: proto.Bool(true),
			Nosuid: proto.Bool(true),
			Nodev:  proto.Bool(true),
		}},
		Hostname: proto.String("app"),
		Cwd:      proto.String("/app"),
		ExecBin: &nsjail.Exe{
			Path: proto.String("/app/run"),
		},
	}
	if cfg.cgroup.cgroup2 {
		msg.UseCgroupv2 = proto.Bool(true)
		msg.Cgroupv2Mount = proto.String(cgroupPath + "/unified/run")
	} else {
		msg.CgroupPidsMount = proto.String(cgroupPath + "/pids")
		msg.CgroupPidsParent = &cfg.cgroup.pids.parent
		msg.CgroupMemMount = proto.String(cgroupPath + "/mem")
		msg.CgroupMemParent = &cfg.cgroup.mem.parent
		msg.CgroupCpuMount = proto.String(cgroupPath + "/cpu")
		msg.CgroupCpuParent = &cfg.cgroup.cpu.parent
	}
	if cfg.Pow != 0 {
		msg.Bindhost = proto.String("127.0.0.1")
		msg.Port = proto.Uint32(cfg.Port + 1)
		msg.MaxConns = proto.Uint32(0)
		msg.MaxConnsPerIp = proto.Uint32(0)
	}
	content, err := prototext.Marshal(msg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(nsjailCfgPath, content, 0644); err != nil {
		return err
	}
	return nil
}

func runNsjail(cfg *jailConfig) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := dropPrivs(cfg); err != nil {
		return err
	}
	if err := unix.Exec("/jail/nsjail", []string{"nsjail", "-C", nsjailCfgPath}, os.Environ()); err != nil {
		return fmt.Errorf("exec nsjail: %w", err)
	}
	return nil
}

func mountTmp() error {
	if err := unix.Mount("", "/tmp", "tmpfs", mountFlags, ""); err != nil {
		return fmt.Errorf("mount tmpfs: %w", err)
	}
	return nil
}

func remountRoot() error {
	if err := unix.Mount("", "/", "", unix.MS_REMOUNT|unix.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("remount root: %w", err)
	}
	return nil
}

func mountDev() error {
	if _, err := os.Stat("/srv/dev"); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := unix.Mount("/jail/dev", "/srv/dev", "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("mount dev: %w", err)
	}
	return nil
}

func runHook() error {
	if _, err := os.Stat(hookPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(
		os.Environ(),
		"cgroup_root="+cgroupPath,
		"nsjail_cfg="+nsjailCfgPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec hook: %w", err)
	}
	return nil
}

func runNsjailChild(errCh chan error) {
	cmd := exec.Command("/jail/nsjail", "-C", nsjailCfgPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		errCh <- fmt.Errorf("run nsjail child: %w", err)
	}
}

func run() error {
	cfg := &jailConfig{}
	if err := env.Parse(cfg); err != nil {
		return fmt.Errorf("parse env config: %w", err)
	}
	if len(os.Args) > 1 && os.Args[1] == "server" {
		errCh := make(chan error)
		go runNsjailChild(errCh)
		go startServer(cfg, errCh)
		return <-errCh
	}
	if cfg.ReadOnly {
		if err := remountRoot(); err != nil {
			return err
		}
	}
	if err := mountTmp(); err != nil {
		return err
	}
	if err := mountDev(); err != nil {
		return err
	}
	cgroup, err := readCgroup()
	if err != nil {
		return err
	}
	cfg.cgroup = cgroup
	if err := mountCgroup(cgroup); err != nil {
		return fmt.Errorf("delegate cgroup: %w", err)
	}
	if err := writeConfig(cfg); err != nil {
		return err
	}
	if err := runHook(); err != nil {
		return err
	}
	if cfg.Pow == 0 {
		if err := runNsjail(cfg); err != nil {
			return err
		}
	} else {
		if err := runServer(cfg); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
