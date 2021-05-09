package main

import (
	"bufio"
	"fmt"
	"github.com/docker/go-units"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"text/template"
)

const (
	cgroupRootPath = "/jail/cgroup"
	nsjailCfgPath  = "/tmp/nsjail.cfg"
	hookPath       = "/jail/hook.sh"
	mountFlags     = uintptr(unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_RELATIME)
	nsjailId       = 1000
)

type cgroup1Entry struct {
	Controllers string
	Parent      string
}

type cgroupInfo struct {
	Pids    *cgroup1Entry
	Mem     *cgroup1Entry
	Cpu     *cgroup1Entry
	Cgroup2 bool
}

type jailConfig struct {
	Time       int64
	Conns      int64
	ConnsPerIp int64
	Pids       int64
	Mem        int64
	Cpu        int64
	Cgroup     cgroupInfo
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
			Controllers: names,
			Parent:      parts[2] + "/NSJAIL",
		}
		switch names {
		case "pids":
			info.Pids = entry
		case "memory":
			info.Mem = entry
		case "cpu", "cpu,cpuacct":
			info.Cpu = entry
		}
	}
	if info.Pids == nil && info.Mem == nil && info.Cpu == nil {
		info.Cgroup2 = true
	}
	return info
}

func mountCgroup1(name string, entry *cgroup1Entry) {
	dest := cgroupRootPath + "/" + name
	if err := unix.Mount("none", dest, "cgroup", mountFlags, entry.Controllers); err != nil {
		panic(fmt.Errorf("mount cgroup1 %s to %s: %w", entry.Controllers, dest, err))
	}
	if err := os.Chmod(dest, 0755); err != nil {
		panic(err)
	}
	delegated := dest + "/" + entry.Parent
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
	tmpl, err := template.New("nsjail").Parse(`
		mode: LISTEN
		port: 5000
		time_limit: {{.Time}}
		max_conns: {{.Conns}}
		max_conns_per_ip: {{.ConnsPerIp}}

		rlimit_as_type: HARD
		rlimit_cpu_type: HARD
		rlimit_fsize_type: HARD
		rlimit_nofile_type: HARD

		cgroup_pids_max: {{.Pids}}
		cgroup_mem_max: {{.Mem}}
		cgroup_cpu_ms_per_sec: {{.Cpu}}

		{{if .Cgroup.Cgroup2}}
			use_cgroupv2: true
			cgroupv2_mount: "/jail/cgroup/unified/run"
		{{else}}
			cgroup_pids_mount: "/jail/cgroup/pids"
			cgroup_pids_parent: "{{.Cgroup.Pids.Parent}}"
			cgroup_mem_mount: "/jail/cgroup/mem"
			cgroup_mem_parent: "{{.Cgroup.Mem.Parent}}"
			cgroup_cpu_mount: "/jail/cgroup/cpu"
			cgroup_cpu_parent: "{{.Cgroup.Cpu.Parent}}"
		{{end}}

		seccomp_string: "ERRNO(1) {"
		seccomp_string: "  clone { (clone_flags & 0x7e020000) != 0 },"
		seccomp_string: "  mount, sethostname, umount, pivot_root"
		seccomp_string: "}"
		seccomp_string: "DEFAULT ALLOW"

		mount {
			src: "/srv"
			dst: "/"
			is_bind: true
			nosuid: true
			nodev: true
		}

		hostname: "app"
		cwd: "/app"
		exec_bin {
			path: "/app/run"
		}
	`)
	if err != nil {
		panic(err)
	}
	file, err := os.Create(nsjailCfgPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if err := tmpl.Execute(file, cfg); err != nil {
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

func readEnv(key string, convert func(string) (int64, error), fallback string) int64 {
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

func convertNum(s string) (int64, error) {
	val, err := strconv.Atoi(s)
	return int64(val), err
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
	if cgroup.Cgroup2 {
		mountCgroup2()
	} else {
		mountCgroup1("pids", cgroup.Pids)
		mountCgroup1("mem", cgroup.Mem)
		mountCgroup1("cpu", cgroup.Cpu)
	}
	writeConfig(&jailConfig{
		Time:       readEnv("JAIL_TIME", convertNum, "30"),
		Conns:      readEnv("JAIL_CONNS", convertNum, "0"),
		ConnsPerIp: readEnv("JAIL_CONNS_PER_IP", convertNum, "0"),
		Pids:       readEnv("JAIL_PIDS", convertNum, "5"),
		Mem:        readEnv("JAIL_MEM", units.RAMInBytes, "5M"),
		Cpu:        readEnv("JAIL_CPU", convertNum, "100"),
		Cgroup:     *cgroup,
	})
	runHook()
	runNsjail()
}
