# redpwn/jail Challenge Author Guide

## Deploying challenges
Here are some best practices and suggestions.

redpwn/jail mounts `/srv` to `/` for each connection, then executes `/app/run` (so `/srv/app/run` outside the jail) with `/app` as the working directory. A common pattern is to copy a "base image" into `/srv` like so:

```dockerfile
FROM pwn.red/jail

# copy / from <some image> to /srv
COPY --from=<some image> / /srv

COPY challenge /srv/app/run
```

This is probably what you want 90% of the time, but it is not required. For example, you may copy a static-linked binary to `/srv/app/run` without copying anything else into `/srv`. This is useful for some simple challenges.

`run` is usually a binary, but any executable is fine. A shell script (with shebang and executable permission set) is a good choice if more flexibility is needed. If you do this, then `/srv` must include a suitable shell. Also, it is good practice to use `exec` whenever possible to reduce the number of processes created.

## Installing dependencies
It is often necessary to install additional libraries or other dependencies. Consider utilizing multi-stage builds to do this:

```dockerfile
FROM python:slim AS app
RUN pip install --no-cache-dir pycryptodome

FROM pwn.red/jail
COPY --from=app / /srv
# ... more ...
```

## Resource limits
redpwn/jail sets fairly strict resource limits by default. It is enough for most challenges, but can be increased if needed. Instructions for doing so are in the [configuration reference](https://github.com/redpwn/jail#configuration-reference).

Notably, challenges written with Python will likely need an increase in memory and process limits. In general, if you find your challenges are hanging or consistently getting killed, then you may need to increase resource limits.

## Use digests!
It is possible that images are updated after a challenge is written. This can cause competitors to have a slightly different setup, which is especially problematic for pwnable challenges that rely heavily on shared libraries. Using image digests ensures that this can not happen.

If your challenge depends on competitors having an identical set of libraries, then:

**Never** do this
```dockerfile
COPY --from=ubuntu / /srv
```

This is *probably* okay:
```dockerfile
COPY --from=ubuntu:jammy-12345678 / /srv
```

This is the best:
```dockerfile
COPY --from=ubuntu@sha256:abcdef0123456789 / /srv
```

There are some challenges where this does not really matter, but providing a specific image reference is still recommended just in case.

## Reference
Read the [configuration reference](../readme.md#configuration-reference) for more information on how to configure redpwn/jail.
