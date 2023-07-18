# redpwn/jail Competitor FAQ

## What is Docker?
Here's [what Docker does](https://docs.docker.com/get-started/). You can download [Docker Desktop here](https://docs.docker.com/get-docker/).

## Why provide Dockerfile instead of just the necessary libraries?
Doing so introduces an extra step that is prone to human error. Challenge authors sometimes update the Dockerfile but forget to update the provided files. Providing the Dockerfile guarantees competitors can run a server identical to the remote server.

## How do I run a server?
Make sure you are in the directory containing `Dockerfile` (or change `.` below to the directory containing `Dockerfile`).

```sh
docker build -t <tag> .
docker run -dp 12345:5000 --privileged <tag>
nc localhost 12345
```

Note the `--privileged` option. You can replace `<tag>` with whatever you want. You can change `12345` to whatever port you want.

## How does the server run the challenge?
When you connect to the server, it mounts `/srv` to `/` and runs `/app/run`. In other words, everything inside of `/srv` becomes the root of the filesystem.

Challenge authors will often write something like this in the Dockerfile:
```dockerfile
COPY --from=ubuntu@sha256:abcdef0123456789 / /srv
```

This means each connection will have whatever is in `ubuntu@sha256:abcdef0123456789` at `/`.

## How do I debug?
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

## What libc/other libraries is the challenge using?
The server mounts `/srv` to `/` for each connection. The challenge uses libraries **under `/srv`**, *not* the libraries under `/`! The library `/lib/libc.so.6` is the libc that redpwn/jail itself uses, and it almost certainly is not the same as the one the challenge is using.

You will find the libraries you want in `/srv/lib`. You can copy these to your local filesystem using the [`docker cp` command](https://docs.docker.com/engine/reference/commandline/cp/).
