package main

import (
	"fmt"

	seccomp "github.com/seccomp/libseccomp-golang"
	"golang.org/x/sys/unix"
)

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

func dropPrivs(cfg *jailConfig) error {
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
	return nil
}
