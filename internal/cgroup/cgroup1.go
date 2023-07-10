package cgroup

import (
	"fmt"
	"os"

	"github.com/redpwn/jail/internal/privs"
	"github.com/redpwn/jail/internal/proto/nsjail"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
)

type cgroup1Entry struct {
	controllers string
}

type cgroup1 struct {
	pids *cgroup1Entry
	mem  *cgroup1Entry
	cpu  *cgroup1Entry
}

func mountCgroup1Entry(name string, entry *cgroup1Entry) error {
	mountPath := rootPath + "/" + name
	if err := unix.Mount("", mountPath, "cgroup", mountFlags, entry.controllers); err != nil {
		return fmt.Errorf("mount cgroup1 %s to %s: %w", entry.controllers, mountPath, err)
	}
	if err := os.Chmod(mountPath, 0755); err != nil {
		return err
	}
	delegated := mountPath + "/" + "NSJAIL"
	if err := os.Mkdir(delegated, 0755); err != nil {
		return err
	}
	if err := os.Chown(delegated, privs.UserId, privs.UserId); err != nil {
		return err
	}
	return nil
}

func (c *cgroup1) Mount() error {
	if err := mountCgroup1Entry("pids", c.pids); err != nil {
		return err
	}
	if err := mountCgroup1Entry("mem", c.mem); err != nil {
		return err
	}
	if err := mountCgroup1Entry("cpu", c.cpu); err != nil {
		return err
	}
	return nil
}

func (c *cgroup1) SetConfig(msg *nsjail.NsJailConfig) {
	msg.CgroupPidsMount = proto.String(rootPath + "/pids")
	msg.CgroupMemMount = proto.String(rootPath + "/mem")
	msg.CgroupCpuMount = proto.String(rootPath + "/cpu")
	if checkExists(rootPath + "/mem/memory.memsw.limit_in_bytes") {
		msg.CgroupMemSwapMax = proto.Int64(0)
	}
}
