# jail

An nsjail Docker image for CTF pwnables. Easily create secure, isolated inetd-style services.

## Features

- Efficiently start a new container-like jail for each incoming TCP connection
- Route each connection to the jail's stdio
- Enforce per-connection CPU/memory/PID/disk resource limits
- Optionally require a proof of work for each connection

## Quick Start

In [`examples/shell`](examples/shell), run:

```sh
sysctl -w kernel.unprivileged_userns_clone=1 # debian <= 10 only
docker-compose up
```

To connect, run:

```sh
nc 127.0.0.1 5000
```

For an example of installing packages inside the jail, [see `examples/cowsay`](examples/cowsay/Dockerfile).

For a Python example with environment configuration, [see `examples/python`](examples/python/Dockerfile).

## Proof of Work

To require a proof of work from clients for every connection, [set `JAIL_POW`](#configuration-reference) to a nonzero difficulty value.
Each difficulty increase of 1500 requires approximately 1 second of CPU time.
The proof of work system is designed to not be parallelizable.

The script [pwn.red/pow](https://pwn.red/pow) downloads, caches, and runs the solver.

## Runtime Reference

The container listens on `JAIL_PORT` (default `5000`) for incoming TCP connections.

Jail must be run as a privileged container.

## Configuration Reference

`/srv` in the container is mounted to `/` in each jail.
Inside each jail, `/app/run` is executed with a working directory of `/app`.

To configure, [use `ENV`](https://docs.docker.com/engine/reference/builder/#env).
To remove a limit, set its value to `0`.

| Name                | Default             | Description                                                        |
| ------------------- | ------------------- | ------------------------------------------------------------------ |
| `JAIL_TIME`         | `20`                | Maximum wall seconds per connection                                |
| `JAIL_CONNS`        | `0`                 | Maximum concurrent connections across all IPs                      |
| `JAIL_CONNS_PER_IP` | `0`                 | Maximum concurrent connections for each IP                         |
| `JAIL_PIDS`         | `5`                 | Maximum PIDs per connection                                        |
| `JAIL_MEM`          | `5M`                | Maximum memory per connection                                      |
| `JAIL_CPU`          | `100`               | Maximum CPU milliseconds per wall second per connection            |
| `JAIL_POW`          | `0`                 | [Proof of work](#proof-of-work) difficulty                         |
| `JAIL_PORT`         | `5000`              | Port number to bind to                                             |
| `JAIL_DEV`          | `null,zero,urandom` | Device files available in `/dev` separated by `,`                  |
| `JAIL_SYSCALLS`     | _(none)_            | Additional allowed syscall names separated by `,`                  |
| `JAIL_TMP_SIZE`     | `0`                 | Maximum size of writable `/tmp` directory in each jail             |
| `JAIL_ENV_*`        | _(none)_            | Environment variables in each jail with `JAIL_ENV_` prefix removed |

If it exists, `/jail/hook.sh` is executed before the jail starts.
Use this script to configure nsjail options or the execution environment.

Files in `JAIL_DEV` are only available if `/srv/dev` exists.
