package config

import (
	"errors"
	"fmt"
	"os"
	"path"

	"golang.org/x/sys/unix"
)

func copyDev(name string) error {
	src := "/dev/" + name
	dst := "/srv/dev/" + name
	stx := &unix.Statx_t{}
	if err := unix.Statx(0, src, 0, unix.STATX_TYPE|unix.STATX_MODE, stx); err != nil {
		return fmt.Errorf("statx %s: %w", src, err)
	}
	t := stx.Mode & unix.S_IFMT
	if t != unix.S_IFBLK && t != unix.S_IFCHR {
		return fmt.Errorf("not block or char device: %s", src)
	}
	if err := os.MkdirAll(path.Dir(dst), 0755); err != nil {
		return err
	}
	if err := unix.Mknod(dst, uint32(stx.Mode), int(unix.Mkdev(stx.Rdev_major, stx.Rdev_minor))); err != nil {
		return fmt.Errorf("mknod %s: %w", dst, err)
	}
	return nil
}

const devMountFlags = uintptr(unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_RELATIME)

func MountDev(devs []string) error {
	if _, err := os.Stat("/srv/dev"); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	oldMask := unix.Umask(0)
	defer unix.Umask(oldMask)
	if err := unix.Mount("", "/srv/dev", "tmpfs", devMountFlags, ""); err != nil {
		return fmt.Errorf("mount dev tmpfs: %w", err)
	}
	for _, n := range devs {
		if err := copyDev(n); err != nil {
			return fmt.Errorf("copy dev %s: %w", n, err)
		}
	}
	if err := unix.Mount("", "/srv/dev", "", unix.MS_REMOUNT|unix.MS_RDONLY|devMountFlags, ""); err != nil {
		return fmt.Errorf("remount dev tmpfs: %w", err)
	}
	return nil
}
