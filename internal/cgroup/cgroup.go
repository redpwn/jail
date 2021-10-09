package cgroup

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/redpwn/jail/internal/proto/nsjail"
	"golang.org/x/sys/unix"
)

type Cgroup interface {
	Mount() error
	SetConfig(*nsjail.NsJailConfig)
}

const (
	rootPath   = "/jail/cgroup"
	mountFlags = uintptr(unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_RELATIME)
)

func ReadCgroup() (Cgroup, error) {
	v1 := &cgroup1{}
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
			v1.pids = entry
		case "memory":
			v1.mem = entry
		case "cpu", "cpu,cpuacct":
			v1.cpu = entry
		}
	}
	if v1.pids == nil && v1.mem == nil && v1.cpu == nil {
		return &cgroup2{}, nil
	}
	return v1, nil
}
