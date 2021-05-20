package main

//go:generate mkdir -p gen
//go:generate protoc -I../nsjail --go_out gen --go_opt Mconfig.proto=/nsjail config.proto

import (
	"bufio"
	"fmt"
	"github.com/docker/go-units"
	"github.com/redpwn/jail/gen/nsjail"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const (
	cgroupRootPath = "/jail/cgroup"
	nsjailCfgPath  = "/tmp/nsjail.cfg"
	hookPath       = "/jail/hook.sh"
	mountFlags     = uintptr(unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_RELATIME)
	nsjailId       = 1000
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

type jailConfig struct {
	time       uint32
	conns      uint32
	connsPerIp uint32
	pids       uint32
	mem        uint32
	cpu        uint32
	cgroup     cgroupInfo
}

func readCgroup() *cgroupInfo {
	info := &cgroupInfo{}
	file, err := os.Open("/proc/self/cgroup")
	if err != nil {
		panic(fmt.Errorf("read cgroup info: %w", err))
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 3)
		names := parts[1]
		entry := &cgroup1Entry{
			controllers: names,
			parent:      parts[2] + "/NSJAIL",
		}
		switch names {
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
	return info
}

func mountCgroup1(name string, entry *cgroup1Entry) {
	dest := cgroupRootPath + "/" + name
	if err := unix.Mount("none", dest, "cgroup", mountFlags, entry.controllers); err != nil {
		panic(fmt.Errorf("mount cgroup1 %s to %s: %w", entry.controllers, dest, err))
	}
	if err := os.Chmod(dest, 0755); err != nil {
		panic(err)
	}
	delegated := dest + "/" + entry.parent
	if err := os.Mkdir(delegated, 0755); err != nil {
		panic(err)
	}
	if err := os.Chown(delegated, nsjailId, nsjailId); err != nil {
		panic(err)
	}
}

func mountCgroup2() {
	dest := cgroupRootPath + "/unified"
	if err := unix.Mount("none", dest, "cgroup2", mountFlags, ""); err != nil {
		panic(fmt.Errorf("mount cgroup2 to %s: %w", dest, err))
	}
	jailPath := dest + "/jail"
	if err := os.Mkdir(jailPath, 0700); err != nil {
		panic(err)
	}
	if err := os.WriteFile(jailPath+"/cgroup.procs", []byte("0"), 0); err != nil {
		panic(err)
	}
	if err := os.WriteFile(dest+"/cgroup.subtree_control", []byte("+pids +memory +cpu"), 0); err != nil {
		panic(err)
	}
	if err := os.Chown(dest+"/cgroup.procs", nsjailId, nsjailId); err != nil {
		panic(fmt.Errorf("cgroup2 delegate: %w", err))
	}
	runPath := dest + "/run"
	if err := os.Mkdir(runPath, 0700); err != nil {
		panic(err)
	}
	if err := os.WriteFile(runPath+"/cgroup.subtree_control", []byte("+pids +memory +cpu"), 0); err != nil {
		panic(err)
	}
	if err := os.Chown(runPath, nsjailId, nsjailId); err != nil {
		panic(err)
	}
}

func writeConfig(cfg *jailConfig) {
	m := nsjail.NsJailConfig{
		Mode:             nsjail.Mode_LISTEN.Enum(),
		Port:             proto.Uint32(5000),
		TimeLimit:        &cfg.time,
		MaxConns:         &cfg.conns,
		MaxConnsPerIp:    &cfg.connsPerIp,
		RlimitAsType:     nsjail.RLimit_HARD.Enum(),
		RlimitCpuType:    nsjail.RLimit_HARD.Enum(),
		RlimitFsizeType:  nsjail.RLimit_HARD.Enum(),
		RlimitNofileType: nsjail.RLimit_HARD.Enum(),
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
		m.UseCgroupv2 = proto.Bool(true)
		m.Cgroupv2Mount = proto.String("/jail/cgroup/unified/run")
	} else {
		m.CgroupPidsMount = proto.String("/jail/cgroup/pids")
		m.CgroupPidsParent = &cfg.cgroup.pids.parent
		m.CgroupMemMount = proto.String("/jail/cgroup/mem")
		m.CgroupMemParent = &cfg.cgroup.mem.parent
		m.CgroupCpuMount = proto.String("/jail/cgroup/cpu")
		m.CgroupCpuParent = &cfg.cgroup.cpu.parent
	}
	c, err := prototext.Marshal(&m)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(nsjailCfgPath, c, 0644); err != nil {
		panic(err)
	}
}

func runNsjail() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := unix.Setresgid(nsjailId, nsjailId, nsjailId); err != nil {
		panic(fmt.Errorf("setresgid nsjail: %w", err))
	}
	if err := unix.Setgroups([]int{nsjailId}); err != nil {
		panic(fmt.Errorf("setgroups nsjail: %w", err))
	}
	if err := unix.Setresuid(nsjailId, nsjailId, nsjailId); err != nil {
		panic(fmt.Errorf("setresuid nsjail: %w", err))
	}
	capHeader := &unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	// https://github.com/golang/go/issues/44312
	capData := [2]unix.CapUserData{}
	if err := unix.Capset(capHeader, &capData[0]); err != nil {
		panic(fmt.Errorf("capset: %w", err))
	}
	if err := unix.Exec("/jail/nsjail", []string{"nsjail", "-C", nsjailCfgPath}, os.Environ()); err != nil {
		panic(fmt.Errorf("exec nsjail: %w", err))
	}
}

func readEnv(key string, convert func(string) (uint32, error), fallback string) uint32 {
	env := os.Getenv(key)
	if env == "" {
		env = fallback
	}
	val, err := convert(env)
	if err != nil {
		panic(fmt.Errorf("read env %s: %w", key, err))
	}
	return val
}

func convertNum(s string) (uint32, error) {
	val, err := strconv.Atoi(s)
	return uint32(val), err
}

func convertSize(s string) (uint32, error) {
	val, err := units.RAMInBytes(s)
	return uint32(val), err
}

func mountTmp() {
	if err := unix.Mount("none", "/tmp", "tmpfs", mountFlags, ""); err != nil {
		panic(fmt.Errorf("mount tmpfs: %w", err))
	}
}

func mountDev() {
	if _, err := os.Stat("/srv/dev"); os.IsNotExist(err) {
		return
	}
	if err := unix.Mount("/jail/dev", "/srv/dev", "none", mountFlags|unix.MS_BIND, ""); err != nil {
		panic(fmt.Errorf("mount dev: %w", err))
	}
}

func runHook() {
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		return
	}
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(
		os.Environ(),
		"cgroup_root="+cgroupRootPath,
		"nsjail_cfg="+nsjailCfgPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("exec hook: %w", err))
	}
}

func main() {
	mountTmp()
	mountDev()
	cgroup := readCgroup()
	if cgroup.cgroup2 {
		mountCgroup2()
	} else {
		mountCgroup1("pids", cgroup.pids)
		mountCgroup1("mem", cgroup.mem)
		mountCgroup1("cpu", cgroup.cpu)
	}
	writeConfig(&jailConfig{
		time:       readEnv("JAIL_TIME", convertNum, "30"),
		conns:      readEnv("JAIL_CONNS", convertNum, "0"),
		connsPerIp: readEnv("JAIL_CONNS_PER_IP", convertNum, "0"),
		pids:       readEnv("JAIL_PIDS", convertNum, "5"),
		mem:        readEnv("JAIL_MEM", convertSize, "5M"),
		cpu:        readEnv("JAIL_CPU", convertNum, "100"),
		cgroup:     *cgroup,
	})
	runHook()
	runNsjail()
}
