FROM debian:11.0-slim AS nsjail
WORKDIR /app
RUN apt-get update && \
  apt-get install -y autoconf bison flex gcc g++ libnl-route-3-dev libprotobuf-dev libseccomp-dev libtool make patch pkg-config protobuf-compiler
COPY nsjail nsjail.patch ./
RUN patch -p1 < nsjail.patch && make -j8

FROM golang:1.17.0-bullseye AS run
WORKDIR /app
RUN apt-get update && apt-get install -y libseccomp-dev libgmp-dev
COPY go.mod go.sum ./
RUN go mod download
COPY cmd cmd
COPY internal internal
RUN go build -v -ldflags '-w -s' ./cmd/jailrun

FROM busybox:1.33.1-glibc
RUN adduser -HDu 1000 jail && \
  mkdir -p /srv /jail/cgroup/cpu /jail/cgroup/mem /jail/cgroup/pids /jail/cgroup/unified /jail/dev && \
  mknod -m 666 /jail/dev/null c 1 3 && \
  mknod -m 666 /jail/dev/zero c 1 5 && \
  mknod -m 444 /jail/dev/urandom c 1 9
COPY --from=nsjail /usr/lib/x86_64-linux-gnu/libprotobuf.so.23 /usr/lib/x86_64-linux-gnu/libnl-route-3.so.200 \
  /lib/x86_64-linux-gnu/libnl-3.so.200 /lib/x86_64-linux-gnu/libz.so.1 /usr/lib/x86_64-linux-gnu/libstdc++.so.6 \
  /lib/x86_64-linux-gnu/libgcc_s.so.1 /lib/
COPY --from=nsjail /app/nsjail /jail/nsjail
COPY --from=run /usr/lib/x86_64-linux-gnu/libseccomp.so.2 /usr/lib/x86_64-linux-gnu/libgmp.so.10 /lib/
COPY --from=run /app/jailrun /jail/run
CMD ["/jail/run"]
