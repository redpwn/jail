package main

import (
	"bufio"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"strconv"
	"strings"
	"text/template"
)

const (
	cgroupRootPath = "/jail/cgroup"
	nsjailCfgPath  = "/tmp/nsjail.cfg"
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
	Time       int
	Conns      int
	ConnsPerIp int
	Pids       int
	Mem        int
	Cpu        int
	Cgroup     cgroupInfo
	Extra      string
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
		if names == "pids" {
			info.Pids = entry
		} else if names == "memory" {
			info.Mem = entry
		} else if names == "cpu" || names == "cpu,cpuacct" {
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

		{{.Extra}}
	`)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(nsjailCfgPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := tmpl.Execute(f, cfg); err != nil {
		panic(err)
	}
}

func runNsjail() {
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
	capData := &unix.CapUserData{}
	if err := unix.Capset(capHeader, capData); err != nil {
		panic(fmt.Errorf("capset: %w", err))
	}
	if err := unix.Exec("/jail/nsjail", []string{"nsjail", "-C", nsjailCfgPath}, []string{}); err != nil {
		panic(fmt.Errorf("exec nsjail: %w", err))
	}
}

func readEnvInt(key string, fallback int) int {
	env := os.Getenv(key)
	if env == "" {
		return fallback
	}
	val, err := strconv.Atoi(env)
	if err != nil {
		panic(fmt.Errorf("read env %s: %w", key, err))
	}
	return val
}

func mountTmp() {
	if err := unix.Mount("none", "/tmp", "tmpfs", mountFlags, ""); err != nil {
		panic(fmt.Errorf("mount tmpfs: %w", err))
	}
}

func mountDev() {
	if data, _ := os.Stat("/srv/dev"); !data.IsDir() {
		return
	}
	if err := unix.Mount("/jail/dev", "/srv/dev", "bind", mountFlags|unix.MS_BIND, ""); err != nil {
		panic(fmt.Errorf("mount dev: %w", err))
	}
}

func readExtra() string {
	data, err := os.ReadFile("/jail/extra.cfg")
	if err != nil {
		return ""
	}
	return string(data)
}

func main() {
	mountTmp()
	mountDev()
	info := readCgroup()
	if info.Cgroup2 {
		mountCgroup2()
	} else {
		mountCgroup1("pids", info.Pids)
		mountCgroup1("mem", info.Mem)
		mountCgroup1("cpu", info.Cpu)
	}
	writeConfig(&jailConfig{
		Time:       readEnvInt("JAIL_TIME", 30),
		Conns:      readEnvInt("JAIL_CONNS", 0),
		ConnsPerIp: readEnvInt("JAIL_CONNS_PER_IP", 0),
		Pids:       readEnvInt("JAIL_PIDS", 5),
		Mem:        readEnvInt("JAIL_MEM", 5242880),
		Cpu:        readEnvInt("JAIL_CPU", 100),
		Extra:      readExtra(),
		Cgroup:     *info,
	})
	runNsjail()
}
