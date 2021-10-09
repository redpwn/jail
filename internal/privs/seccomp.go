package privs

import (
	"fmt"

	"github.com/redpwn/jail/internal/config"
	seccomp "github.com/seccomp/libseccomp-golang"
	"golang.org/x/sys/unix"
)

func initSeccomp(cfg *config.Config) error {
	arch, err := seccomp.GetNativeArch()
	if err != nil {
		return err
	}
	if arch != seccomp.ArchAMD64 {
		return fmt.Errorf("native arch %s is not amd64", arch)
	}
	defaultAct := seccomp.ActErrno.SetReturnCode(int16(unix.EPERM))
	filter, err := seccomp.NewFilter(defaultAct)
	if err != nil {
		return err
	}
	if err := filter.AddArch(seccomp.ArchX86); err != nil {
		return err
	}

	for _, rule := range seccompRules {
		for _, name := range rule.names {
			call, err := seccomp.GetSyscallFromName(name)
			if err != nil {
				// match runc behavior for builtin syscalls
				// https://github.com/opencontainers/runc/blob/c61f6062547d20b80a07e9593e9617e115773b28/libcontainer/seccomp/seccomp_linux.go#L154-L159
				continue
			}
			if err := filter.AddRule(call, rule.act); err != nil {
				return err
			}
		}
	}

	for _, name := range cfg.Syscalls {
		call, err := seccomp.GetSyscallFromName(name)
		if err != nil {
			return err
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
