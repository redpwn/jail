package cgroup

import (
	"fmt"
	"os"

	"github.com/redpwn/jail/internal/privs"
	"github.com/redpwn/jail/internal/proto/nsjail"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"
)

type cgroup2 struct {
	parent string
}

func (c *cgroup2) Mount() error {
	mountPath := rootPath + "/unified"
	if err := unix.Mount("", mountPath, "cgroup2", mountFlags, ""); err != nil {
		return fmt.Errorf("mount cgroup2 to %s: %w", mountPath, err)
	}
	parentPath := mountPath + c.parent
	jailPath := parentPath + "/jail"
	if err := os.Mkdir(jailPath, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(jailPath+"/cgroup.procs", []byte("0"), 0); err != nil {
		return err
	}
	if err := os.WriteFile(parentPath+"/cgroup.subtree_control", []byte("+pids +memory +cpu"), 0); err != nil {
		return err
	}
	if err := os.Chown(parentPath+"/cgroup.procs", privs.UserId, privs.UserId); err != nil {
		return err
	}
	runPath := parentPath + "/run"
	if err := os.Mkdir(runPath, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(runPath+"/cgroup.subtree_control", []byte("+pids +memory +cpu"), 0); err != nil {
		return err
	}
	if err := os.Chown(runPath, privs.UserId, privs.UserId); err != nil {
		return err
	}
	return nil
}

func (c *cgroup2) SetConfig(msg *nsjail.NsJailConfig) {
	msg.UseCgroupv2 = proto.Bool(true)
	msg.Cgroupv2Mount = proto.String(rootPath + "/unified/" + c.parent + "/run")
	if checkExists(rootPath + "/unified/memory.swap.max") {
		msg.CgroupMemSwapMax = proto.Int64(0)
	}
}
