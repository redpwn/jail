#!/bin/sh

set -eu

mount -t tmpfs tmpfs /tmp

cgroup_root=/jail/cgroup

mount_cgroup() {
  mount -t cgroup -o "$1,rw,nosuid,nodev,noexec,relatime" cgroup "$cgroup_root/$2" || return 1
  chmod u+w "$cgroup_root/$2"
  mkdir -p "$cgroup_root/$2/NSJAIL"
  chown nsjail:nsjail "$cgroup_root/$2/NSJAIL"
}

mount_cgroup pids pids
mount_cgroup memory memory
mount_cgroup cpu cpu || mount_cgroup cpu,cpuacct cpu

nsjail_cfg=/tmp/nsjail.cfg

cat << EOF > $nsjail_cfg
mode: LISTEN
port: 5000
time_limit: ${JAIL_WALL_TIME:-30}
max_conns: ${JAIL_CONNS:-0}
max_conns_per_ip: ${JAIL_CONNS_PER_IP:-0}

rlimit_as_type: HARD
rlimit_cpu_type: HARD
rlimit_fsize_type: HARD
rlimit_nofile_type: HARD
cgroup_pids_max: ${JAIL_PIDS:-5}
cgroup_pids_mount: "$cgroup_root/pids"
cgroup_mem_max: ${JAIL_MEM:-5242880}
cgroup_mem_mount: "$cgroup_root/memory"
cgroup_cpu_ms_per_sec: ${JAIL_CPU:-100}
cgroup_cpu_mount: "$cgroup_root/cpu"
max_cpus: 1

seccomp_string: "KILL {"
seccomp_string: "  clone { (clone_flags & 0x7e020000) != 0 },"
seccomp_string: "  mount, sethostname, umount, pivot_root"
seccomp_string: "}"
seccomp_string: "DEFAULT ALLOW"

mount {
  src: "/app/app"
  dst: "/app"
  is_bind: true
}
mount {
  src: "/app/bin"
  dst: "/bin"
  is_bind: true
}
mount {
  src: "/app/usr/bin"
  dst: "/usr/bin"
  is_bind: true
}
mount {
  src: "/app/lib/x86_64-linux-gnu"
  dst: "/lib/x86_64-linux-gnu"
  is_bind: true
}
mount {
  src: "/app/lib64"
  dst: "/lib64"
  is_bind: true
}
mount {
  src: "/dev/urandom"
  dst: "/dev/urandom"
  is_bind: true
}
mount {
  src: "/dev/null"
  dst: "/dev/null"
  is_bind: true
}
mount {
  src: "/dev/zero"
  dst: "/dev/zero"
  is_bind: true
}

hostname: "challenge"
cwd: "/app"
exec_bin {
  path: "/app/challenge"
}
EOF

[ -e /jail/hook.sh ] && . /jail/hook.sh

exec setuidgid nsjail setpriv --inh-caps -chown,-setuid,-setgid,-sys_admin,-setpcap /jail/nsjail -C $nsjail_cfg
