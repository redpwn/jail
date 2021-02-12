# [redpwn/jail](https://hub.docker.com/r/redpwn/jail)

An [nsjail](https://nsjail.dev/) Docker image for CTF pwnables

Usage example:

```dockerfile
FROM redpwn/jail

COPY --from=ubuntu:focal / /app
COPY flag.txt /app/app/flag.txt
COPY binary /app/app/challenge
```

On Debian, set `kernel.unprivileged_userns_clone=1` before running the container.

Refer to the [docker-compose.yml](https://github.com/redpwn/jail/blob/master/docker-compose.yml)
for runtime options (capabilities, seccomp, AppArmor, etc.) required to run
this container.
