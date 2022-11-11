# jail
An nsjail Docker image for CTF pwnables. Easily create secure, isolated inetd-style services.

> Playing a CTF that uses redpwn/jail? Skip to [the Competitor FAQ](#competitor-faq).

## Features
- Efficiently start a new container-like jail for each incoming TCP connection
- Route each connection to the jail's stdio
- Enforce per-connection CPU/memory/PID/disk resource limits
- Optionally require a proof of work for each connection

## Quick start
In [`examples/shell`](examples/shell), run:

```sh
sysctl -w kernel.unprivileged_userns_clone=1 # debian <= 10 only
docker-compose up
```

To connect, run:

```sh
nc localhost 5000
```

For an example of installing packages inside the jail, [see `examples/cowsay`](examples/cowsay/Dockerfile).

For a Python example with environment configuration, [see `examples/python`](examples/python/Dockerfile).

## Guide for challenge authors
[redpwn/jail](https://github.com/redpwn/jail) is a Docker image based on nsjail that makes it super easy to deploy pwnables and other types of services for CTF competitions.

### Background
Turning an executable into a networked service is not an easy task. Traditionally, this involved a rather long Dockerfile based on xinetd, which has a very limited feature set. Ideally, we would want complete isolation between connections, and be able to set strict limits so resources are not exhausted by a single competitor (intentionally or otherwise).

Many challenges, particularly pwnables, will result in remote code execution. This makes isolation even more challenging.

[nsjail](https://github.com/google/nsjail) provides the server and strong isolation we need. Unfortunately, nsjail has many configuration options and would be a pain to configure for every challenge. redpwn/jail is essentially a wrapper around nsjail that uses a sensible default configuration for CTF challenges and exposes a small set of options that a CTF challenge may require. It also includes a proof-of-work system that can be enabled with one environment variable.

### Deploying challenges
Here are some best practices and suggestions. Jump to the [configuration reference](#configuration-reference).

redpwn/jail mounts `/srv` to `/` for each connection, then executes `/app/run` (so `/srv/app/run` outside the jail) with `/app` as the working directory. A common pattern is to copy a "base image" into `/srv` like so:

```dockerfile
FROM pwn.red/jail

# copy / from <some image> to /srv
COPY --from=<some image> / /srv

COPY challenge /srv/app/run
```

This is probably what you want 90% of the time, but it is not required. For example, you may copy a static-linked binary to `/srv/app/run` without copying anything else into `/srv`. This is useful for some simple challenges.

`run` is usually a binary, but any executable is fine. A shell script (with shebang and executable permission) is a good choice if more flexibility is needed. If you do this, then `/srv` must include a suitable shell. Also, it is good practice to use `exec` whenever possible to reduce the number of processes created.

#### Installing dependencies
It is often necessary to install additional libraries or other dependencies. Consider utilizing multi-stage builds to do this:

```dockerfile
FROM python:slim AS app
RUN pip install --no-cache-dir pycryptodome

FROM pwn.red/jail
COPY --from=app / /srv
# ... more ...
```

#### Resource limits
redpwn/jail sets fairly strict resource limits by default. It is enough for most challenges, but can be increased if needed. Instructions for doing so are in the [configuration reference](https://github.com/redpwn/jail#configuration-reference).

Notably, challenges written with Python will likely need an increase in memory and process limits. In general, if you find your challenges are hanging or consistently getting killed, then you may need to increase resource limits.

#### Use digests!
It is possible that images are updated after a challenge is written. This can cause competitors to have a slightly different setup, which is especially problematic for pwnable challenges that rely heavily on shared libraries. Using image digests ensures that this can not happen.

If your challenge depends on competitors having an identical set of libraries, then:

**Never** do this
```dockerfile
COPY --from=ubuntu / /srv
```

This is *probably* okay
```dockerfile
COPY --from=ubuntu:jammy-12345678 / /srv
```

This is the best
```dockerfile
COPY --from=ubuntu@sha256:abcdef0123456789 / /srv
```

There are some challenges where this does not really matter, but providing a somewhat-specific tag is still recommended just in case.

## Competitor FAQ
Here are some questions we get a lot.

### What is Docker?
Here's [what Docker does](https://docs.docker.com/get-started/). Download [Docker desktop here](https://docs.docker.com/get-docker/).

### Why provide Dockerfile instead of just the necessary libraries?
Doing so introduces an extra step that is prone to human error. Multiple times, authors will update the Dockerfile but forget to update the provided files. Providing the Dockerfile guarantees competitors can run a server identical to the remote server.

### How do I run a server?
Make sure you are in the directory containing `Dockerfile` (or change `.` below to the directory containing `Dockerfile`).

```sh
docker build -t <tag> .
docker run -dp 12345:5000 --privileged <tag>
nc localhost 12345
```

Note the `--privileged` option. You can replace `<tag>` with whatever you want. You can change `12345` to whatever port you want.

You may also want some additional options:

```sh
docker run -dp 12345:5000 --privileged --rm --name <name> <tag>
```

`--rm` automatically removes the container when it exits, and `--name` gives the container a name you can use to, for example, stop it:

```sh
docker stop <name>
```

### How does the server run the challenge?
When you connect to the server, it mounts `/srv` to `/` and runs `/app/run`. In other words, everything inside of `/srv` becomes the root of the filesystem.

Challenge authors will often write something like this in the Dockerfile:
```dockerfile
COPY --from=ubuntu@sha256:abcdef0123456789 / /srv
```

This means each connection will have whatever is in `ubuntu@sha256:abcdef0123456789` at `/`.

### How do I debug?
The strong isolation that redpwn/jail provides makes it difficult to debug directly. It is often possible (and easier) to solve a challenge by simply using the tools installed on your machine and not debugging inside of the container.

If you feel you must debug inside of a container, then you can create a new image with only what is inside `/srv`. This is usually good enough.

Challenge authors will often write something like this in the Dockerfile:
```dockerfile
COPY --from=ubuntu@sha256:abcdef0123456789 / /srv
```

You can start your new image with:
```dockerfile
FROM ubuntu@sha256:abcdef0123456789
```

Then, add any challenge files you need and install whatever tools you prefer.

### What libc/other libraries is the challenge using?
The server mounts `/srv` to `/` for each connection. The challenge uses libraries **under `/srv`**, *not* the libraries under `/`! The library `/lib/libc.so.6` is the libc that redpwn/jail itself uses, and it almost certainly is not the same as the one the challenge is using.

You will find the libraries you want in `/srv/lib`. You can copy these to your local filesystem using the [`docker cp` command](https://docs.docker.com/engine/reference/commandline/cp/).

## Configuration Reference
`/srv` in the container is mounted to `/` in each jail. Inside each jail, `/app/run` is executed with a working directory of `/app`.

To configure, [use `ENV`](https://docs.docker.com/engine/reference/builder/#env). To remove a limit, set its value to `0`.

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

If it exists, `/jail/hook.sh` is executed before the jail starts. Use this script to configure nsjail options or the execution environment.

Files in `JAIL_DEV` are only available if `/srv/dev` exists.

### Proof of Work
To require a proof of work from clients for every connection, [set `JAIL_POW`](#configuration-reference) to a nonzero difficulty value. Each difficulty increase of 1500 requires approximately 1 second of CPU time. The proof of work system is designed to not be parallelizable.

The script [pwn.red/pow](https://pwn.red/pow) downloads, caches, and runs the solver.
