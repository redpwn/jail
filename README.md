# [redpwn/jail](https://hub.docker.com/r/redpwn/jail)

An [nsjail](https://nsjail.dev) Docker image for CTF pwnables

## Quick Start

In [`examples/shell`](https://github.com/redpwn/jail/tree/master/examples/shell), run:

```bash
sysctl -w kernel.unprivileged_userns_clone=1 # debian only
docker-compose up
```

To connect to the jail, run:

```bash
nc 127.0.0.1 5000
```

For an example of installing packages inside the jail, [see `examples/cowsay`](https://github.com/redpwn/jail/blob/master/examples/cowsay/Dockerfile).

For a Python example with environment configuration, [see `examples/python`](https://github.com/redpwn/jail/blob/master/examples/python/Dockerfile).

## Runtime Reference

Jails require some container security options.
The shell example [`docker-compose.yml`](https://github.com/redpwn/jail/blob/master/examples/shell/docker-compose.yml) specifies these options.

* AppArmor: `unconfined`
* [seccomp: `seccomp.json`](https://github.com/redpwn/jail/blob/master/seccomp.json)
* Capabilities: `chown`, `setuid`, `setgid`, `sys_admin`, `setpcap`

Jails are not compatible with SELinux.
## Configuration Reference

`/srv` outside the jail is mounted to `/` inside the jail.
Inside each jail, `/app/run` is executed with a working directory of `/app`.

To configure a limit, [use `ENV`](https://docs.docker.com/engine/reference/builder/#env).
To remove a limit, set its value to `0`.

Name|Default|Description
-|-|-
`JAIL_TIME`|30|Maximum wall seconds per connection
`JAIL_CONNS`|0|Maximum concurrent connections across all IPs
`JAIL_CONNS_PER_IP`|0|Maximum concurrent connections for each IP
`JAIL_PIDS`|5|Maximum PIDs per connection
`JAIL_MEM`|5242880|Maximum memory bytes per connection
`JAIL_CPU`|100|Maximum CPU milliseconds per wall second per connection

If it exists, `/jail/hook.sh` is sourced before the jail starts.
Use this script to configure nsjail options or the execution environment.
