# redpwn/jail

An [nsjail](https://nsjail.dev) Docker image for CTF pwnables

## Quick Start

In [`examples/shell`](https://github.com/redpwn/jail/tree/master/examples/shell), run:

```sh
sysctl -w kernel.unprivileged_userns_clone=1 # debian <= 10 only
docker-compose up
```

To connect to the jail, run:

```sh
nc 127.0.0.1 5000
```

For an example of installing packages inside the jail, [see `examples/cowsay`](https://github.com/redpwn/jail/blob/master/examples/cowsay/Dockerfile).

For a Python example with environment configuration, [see `examples/python`](https://github.com/redpwn/jail/blob/master/examples/python/Dockerfile).

## Proof of Work

To require a proof of work from clients for every connection, [set `JAIL_POW`](#configuration-reference) to a nonzero difficulty value.
Each difficulty increase of 1500 requires approximately 1 second of CPU time.
The proof of work system is designed to not be parallelizable.

The script [pwn.red/pow](https://pwn.red/pow) downloads, caches, and runs the solver.

## Runtime Reference

The container listens on `JAIL_PORT` (default `5000`) for incoming TCP connections.

Jails require some container security options.
The example [`docker-compose.yml`](https://github.com/redpwn/jail/blob/master/examples/shell/docker-compose.yml) specifies these options.

- AppArmor: `unconfined`
- seccomp: `unconfined`
- Capabilities: `chown`, `setuid`, `setgid`, `sys_admin`

Jails are not compatible with SELinux.

## Configuration Reference

`/srv` outside the jail is mounted to `/` inside the jail.
Inside each jail, `/app/run` is executed with a working directory of `/app`.

To configure, [use `ENV`](https://docs.docker.com/engine/reference/builder/#env).
To remove a limit, set its value to `0`.

| Name                | Default             | Description                                             |
| ------------------- | ------------------- | ------------------------------------------------------- |
| `JAIL_TIME`         | `20`                | Maximum wall seconds per connection                     |
| `JAIL_CONNS`        | `0`                 | Maximum concurrent connections across all IPs           |
| `JAIL_CONNS_PER_IP` | `0`                 | Maximum concurrent connections for each IP              |
| `JAIL_PIDS`         | `5`                 | Maximum PIDs per connection                             |
| `JAIL_MEM`          | `5M`                | Maximum memory per connection                           |
| `JAIL_CPU`          | `100`               | Maximum CPU milliseconds per wall second per connection |
| `JAIL_POW`          | `0`                 | [Proof of work](#proof-of-work) difficulty              |
| `JAIL_PORT`         | `5000`              | Port number to bind to                                  |
| `JAIL_DEV`          | `null,zero,urandom` | Device files available in `/dev` separated by `,`       |
| `JAIL_SYSCALLS`     | _(none)_            | Additional allowed syscall names separated by `,`       |
| `JAIL_TMP_SIZE`     | `0`                 | Maximum size of writable `/tmp` directory               |

If it exists, `/jail/hook.sh` is executed before the jail starts.
Use this script to configure nsjail options or the execution environment.

Files in `JAIL_DEV` are only available if `/srv/dev` exists.
