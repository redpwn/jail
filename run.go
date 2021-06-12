package main

//go:generate mkdir -p proto
//go:generate protoc -Insjail --go_out proto --go_opt Mconfig.proto=/nsjail config.proto

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/caarlos0/env/v6"
	"github.com/docker/go-units"
	"github.com/redpwn/jail/proto/nsjail"
	seccomp "github.com/seccomp/libseccomp-golang"
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

type cgroup1Entry struct {
	controllers string
	parent      string
}

type cgroupInfo struct {
	pids    *cgroup1Entry
	mem     *cgroup1Entry
	cpu     *cgroup1Entry
	cgroup2 bool
}

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
	Port       uint32   `env:"JAIL_PORT" envDefault:"5000"`
	Syscalls   []string `env:"JAIL_SYSCALLS"`
	ReadOnly   bool     `env:"JAIL_READ_ONLY" envDefault:"true"`
	cgroup     cgroupInfo
}

func readCgroup() (*cgroupInfo, error) {
	info := &cgroupInfo{}
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return nil, fmt.Errorf("read cgroup info: %w", err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		parts := strings.SplitN(s.Text(), ":", 3)
		entry := &cgroup1Entry{
			controllers: parts[1],
			parent:      parts[2] + "/NSJAIL",
		}
		switch parts[1] {
		case "pids":
			info.pids = entry
		case "memory":
			info.mem = entry
		case "cpu", "cpu,cpuacct":
			info.cpu = entry
		}
	}
	if info.pids == nil && info.mem == nil && info.cpu == nil {
		info.cgroup2 = true
	}
	return info, nil
}

func mountCgroup1(name string, entry *cgroup1Entry) error {
	dest := cgroupPath + "/" + name
	if err := unix.Mount("", dest, "cgroup", mountFlags, entry.controllers); err != nil {
		return fmt.Errorf("mount cgroup1 %s to %s: %w", entry.controllers, dest, err)
	}
	if err := os.Chmod(dest, 0755); err != nil {
		return err
	}
	delegated := dest + "/" + entry.parent
	if err := os.Mkdir(delegated, 0755); err != nil {
		return err
	}
	if err := os.Chown(delegated, nsjailId, nsjailId); err != nil {
		return err
	}
	return nil
}

func mountCgroup2() error {
	dest := cgroupPath + "/unified"
	if err := unix.Mount("", dest, "cgroup2", mountFlags, ""); err != nil {
		return fmt.Errorf("mount cgroup2 to %s: %w", dest, err)
	}
	jailPath := dest + "/jail"
	if err := os.Mkdir(jailPath, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(jailPath+"/cgroup.procs", []byte("0"), 0); err != nil {
		return err
	}
	if err := os.WriteFile(dest+"/cgroup.subtree_control", []byte("+pids +memory +cpu"), 0); err != nil {
		return err
	}
	if err := os.Chown(dest+"/cgroup.procs", nsjailId, nsjailId); err != nil {
		return fmt.Errorf("cgroup2 delegate: %w", err)
	}
	runPath := dest + "/run"
	if err := os.Mkdir(runPath, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(runPath+"/cgroup.subtree_control", []byte("+pids +memory +cpu"), 0); err != nil {
		return err
	}
	if err := os.Chown(runPath, nsjailId, nsjailId); err != nil {
		return err
	}
	return nil
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
	content, err := prototext.Marshal(msg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(nsjailCfgPath, content, 0644); err != nil {
		return err
	}
	return nil
}

func initSeccomp(cfg *jailConfig) error {
	arch, err := seccomp.GetNativeArch()
	if err != nil {
		return err
	}
	if arch != seccomp.ArchAMD64 {
		return fmt.Errorf("native arch %s is not amd64", arch)
	}
	act := seccomp.ActErrno.SetReturnCode(int16(unix.EPERM))
	filter, err := seccomp.NewFilter(act)
	if err != nil {
		return err
	}
	if err := filter.AddArch(seccomp.ArchX86); err != nil {
		return err
	}
	for i, name := range append(seccompSyscalls, cfg.Syscalls...) {
		call, err := seccomp.GetSyscallFromName(name)
		if err != nil {
			// return error for invalid custom syscall names
			if i >= len(seccompSyscalls) {
				return err
			}

			// match runc behavior for builtin syscalls
			// https://github.com/opencontainers/runc/blob/c61f6062547d20b80a07e9593e9617e115773b28/libcontainer/seccomp/seccomp_linux.go#L154-L159
			continue
		}
		if err := filter.AddRule(call, seccomp.ActAllow); err != nil {
			return err
		}
	}
	if err := filter.Load(); err != nil {
		return err
	}
	return nil
}

func runNsjail(cfg *jailConfig) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := initSeccomp(cfg); err != nil {
		return fmt.Errorf("init seccomp: %w", err)
	}
	if err := unix.Setresgid(nsjailId, nsjailId, nsjailId); err != nil {
		return fmt.Errorf("setresgid nsjail: %w", err)
	}
	if err := unix.Setgroups([]int{nsjailId}); err != nil {
		return fmt.Errorf("setgroups nsjail: %w", err)
	}
	if err := unix.Setresuid(nsjailId, nsjailId, nsjailId); err != nil {
		return fmt.Errorf("setresuid nsjail: %w", err)
	}
	capHeader := &unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	// https://github.com/golang/go/issues/44312
	capData := [2]unix.CapUserData{}
	if err := unix.Capset(capHeader, &capData[0]); err != nil {
		return fmt.Errorf("capset: %w", err)
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

func remountRoot() error {
	if err := unix.Mount("", "/", "", unix.MS_REMOUNT|unix.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("remount root: %w", err)
	}
	return nil
}

func mountCgroup(info *cgroupInfo) error {
	if info.cgroup2 {
		if err := mountCgroup2(); err != nil {
			return err
		}
	} else {
		if err := mountCgroup1("pids", info.pids); err != nil {
			return err
		}
		if err := mountCgroup1("mem", info.mem); err != nil {
			return err
		}
		if err := mountCgroup1("cpu", info.cpu); err != nil {
			return err
		}
	}
	return nil
}

func run() error {
	cfg := &jailConfig{}
	if err := env.Parse(cfg); err != nil {
		return fmt.Errorf("parse env config: %w", err)
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
	cfg.cgroup = *cgroup
	if err := mountCgroup(cgroup); err != nil {
		return fmt.Errorf("delegate cgroup: %w", err)
	}
	if err := writeConfig(cfg); err != nil {
		return err
	}
	if err := runHook(); err != nil {
		return err
	}
	if err := runNsjail(cfg); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
