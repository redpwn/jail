# redpwn/jail
An nsjail Docker image for CTF pwnables. Easily create secure, isolated xinetd/inetd-style services.

> Playing a CTF that uses redpwn/jail? Read [the Competitor FAQ](docs/competitors.md).

> Want to use redpwn/jail in your CTF? Read the [the Challenge Author Guide](docs/challenge-authors.md).

## Features
- Efficiently start a new container-like jail for each incoming TCP connection
- Route each connection to the jail's stdio
- Enforce per-connection CPU/memory/PID/disk resource limits
- Optionally require a proof of work for each connection

## Quick start
Create a `Dockerfile`:
```dockerfile
# use the jail base image
FROM pwn.red/jail
# copy the root files from any Docker image
COPY --from=ubuntu / /srv
# setup the binary to run
RUN mkdir /srv/app && ln -s /bin/sh /srv/app/run
```

Then, build and run the container with:
```sh
docker run -p 5000:5000 --privileged $(docker build -q .)
```

To connect, run:
```sh
nc localhost 5000
```

You're now connected to a shell that's fully sandboxed by redpwn/jail! You can run any command you like.

For an example of installing packages inside the jail, [see `examples/cowsay`](examples/cowsay/Dockerfile).

For a Python example with environment configuration, [see `examples/python`](examples/python/Dockerfile).

## Background
Turning an executable into a sandboxed network service is not an easy task. Traditionally, this involved a long Dockerfile based on xinetd, which has a very limited feature set. Ideally, we would also want complete isolation between connections and be able to set strict limits so resources are not exhausted by a single competitor. Many challenges, particularly pwnables, will result in remote code execution. This makes isolation even more challenging.

Google's [nsjail](https://github.com/google/nsjail) provides the server and strong isolation we need. Unfortunately, nsjail provides limited defense-in-depth, has many complex options, and doesn't work in every environment. redpwn/jail is a wrapper around nsjail with sensible default configuration for CTF challenges that exposes a small set of options a CTF challenge may require. It also includes a proof-of-work system that can be enabled with one environment variable.

## Configuration Reference
> For an overview of using redpwn/jail in a CTF, read [the Challenge Author Guide](docs/challenge-authors.md).

redpwn/jail mounts `/srv` in the container to `/` in each jail, then executes `/app/run` (so `/srv/app/run` outside the jail) with a working directory of `/app`.

To configure these, [use `ENV`](https://docs.docker.com/engine/reference/builder/#env) in your Dockerfile. To remove a limit, set its value to `0`.

| Name                | Default             | Description                                                                                                                 |
| ------------------- | ------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `JAIL_TIME`         | `20`                | Maximum wall seconds per connection                                                                                         |
| `JAIL_CONNS`        | `0`                 | Maximum concurrent connections across all IPs                                                                               |
| `JAIL_CONNS_PER_IP` | `0`                 | Maximum concurrent connections for each IP                                                                                  |
| `JAIL_PIDS`         | `5`                 | Maximum PIDs in use per connection                                                                                          |
| `JAIL_MEM`          | `5M`                | Maximum memory per connection                                                                                               |
| `JAIL_CPU`          | `100`               | Maximum CPU milliseconds per wall second per connection. For example, `100` means each connection can use 10% of a CPU core |
| `JAIL_POW`          | `0`                 | [Proof of work](#proof-of-work) difficulty                                                                                  |
| `JAIL_PORT`         | `5000`              | Port number to bind to                                                                                                      |
| `JAIL_DEV`          | `null,zero,urandom` | Device files available in `/dev` separated by `,`                                                                           |
| `JAIL_SYSCALLS`     | _(none)_            | Additional allowed syscall names separated by `,`                                                                           |
| `JAIL_TMP_SIZE`     | `0`                 | Maximum size of writable `/tmp` directory in each jail. If set to `0`, the writable `/tmp` directory is unavailable.        |
| `JAIL_ENV_*`        | _(none)_            | Environment variables available in each jail (with the `JAIL_ENV_` prefix removed)                                          |

If it exists, `/jail/hook.sh` is executed before the jail starts. Use this script to configure nsjail options or the execution environment.

Files specified in `JAIL_DEV` are only available if `/srv/dev` exists.

In each jail, procfs is only mounted to `/proc` if `/srv/proc` exists.

### Proof of Work
To require a proof of work from clients for every connection, [set `JAIL_POW`](#configuration-reference) to a nonzero difficulty value. Each difficulty increase of 1500 requires approximately 1 second of CPU time on a modern processor. The proof of work system is designed to not be parallelizable.

End users are instructed to use the script at [pwn.red/pow](https://pwn.red/pow) to download, cache, and run a prebuilt solver.
