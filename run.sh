#!/bin/sh

set -eu

cgroup_root=/jail/cgroup
nsjail_cfg=/tmp/nsjail.cfg

mount -t tmpfs -o rw,nosuid,nodev,noexec,relatime tmpfs /tmp

[ -d /srv/dev ] && mount --bind /jail/dev /srv/dev

mount_cgroup() {
  mount -t cgroup -o "$1,rw,nosuid,nodev,noexec,relatime" cgroup "$cgroup_root/$2" || return 1
  chmod u+w "$cgroup_root/$2"
  parent=$(awk -v "controller=$1" '{split($0, parts, ":")} parts[2] == controller {print parts[3]}' /proc/self/cgroup)/NSJAIL
  mkdir -p "$cgroup_root/$2/$parent"
  chown nsjail:nsjail "$cgroup_root/$2/$parent"
  echo "$parent"
}

cat << EOF > $nsjail_cfg
mode: LISTEN
port: 5000
time_limit: ${JAIL_TIME:-30}
max_conns: ${JAIL_CONNS:-0}
max_conns_per_ip: ${JAIL_CONNS_PER_IP:-0}

rlimit_as_type: HARD
rlimit_cpu_type: HARD
rlimit_fsize_type: HARD
rlimit_nofile_type: HARD
cgroup_pids_max: ${JAIL_PIDS:-5}
cgroup_pids_mount: "$cgroup_root/pids"
cgroup_pids_parent: "$(mount_cgroup pids pids)"
cgroup_mem_max: ${JAIL_MEM:-5242880}
cgroup_mem_mount: "$cgroup_root/memory"
cgroup_mem_parent: "$(mount_cgroup memory memory)"
cgroup_cpu_ms_per_sec: ${JAIL_CPU:-100}
cgroup_cpu_mount: "$cgroup_root/cpu"
cgroup_cpu_parent: "$(mount_cgroup cpu,cpuacct cpu || mount_cgroup cpu cpu)"

seccomp_string: "KILL {"
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
EOF

# Ignore shellcheck not being able to find the sourced file
# shellcheck disable=SC1091
[ -e /jail/hook.sh ] && . /jail/hook.sh

exec setuidgid nsjail setpriv --inh-caps -chown,-setuid,-setgid,-sys_admin,-setpcap /jail/nsjail -C $nsjail_cfg
