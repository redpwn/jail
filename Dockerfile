# syntax=docker/dockerfile:1.5.2

FROM debian:bookworm-20230703-slim AS nsjail
WORKDIR /app
RUN apt-get update && \
  apt-get install -y autoconf bison flex gcc g++ libnl-route-3-dev libprotobuf-dev libseccomp-dev libtool make pkg-config protobuf-compiler
COPY nsjail .
RUN make -j

FROM golang:1.20.6-bookworm AS run
WORKDIR /app
RUN apt-get update && apt-get install -y libseccomp-dev libgmp-dev
COPY go.mod go.sum ./
RUN go mod download
COPY cmd cmd
COPY internal internal
RUN go build -v -ldflags '-w -s' ./cmd/jailrun

FROM busybox:1.36.1-glibc AS image
RUN adduser -HDu 1000 jail && \
  mkdir -p /srv /jail/cgroup/cpu /jail/cgroup/mem /jail/cgroup/pids /jail/cgroup/unified
COPY --link --from=nsjail /usr/lib/*-linux-gnu/libprotobuf.so.32 /usr/lib/*-linux-gnu/libnl-route-3.so.200 \
  /lib/*-linux-gnu/libnl-3.so.200 /lib/*-linux-gnu/libz.so.1 /usr/lib/*-linux-gnu/libstdc++.so.6 \
  /lib/*-linux-gnu/libgcc_s.so.1 /lib/
COPY --link --from=run /usr/lib/*-linux-gnu/libseccomp.so.2 /usr/lib/*-linux-gnu/libgmp.so.10 /lib/
COPY --link --from=nsjail /app/nsjail /jail/nsjail
COPY --link --from=run /app/jailrun /jail/run

FROM scratch
COPY --from=image / /
CMD ["/jail/run"]
