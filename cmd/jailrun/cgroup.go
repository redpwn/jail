package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
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
	if err := os.Chown(delegated, userId, userId); err != nil {
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
	if err := os.Chown(dest+"/cgroup.procs", userId, userId); err != nil {
		return fmt.Errorf("cgroup2 delegate: %w", err)
	}
	runPath := dest + "/run"
	if err := os.Mkdir(runPath, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(runPath+"/cgroup.subtree_control", []byte("+pids +memory +cpu"), 0); err != nil {
		return err
	}
	if err := os.Chown(runPath, userId, userId); err != nil {
		return err
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
