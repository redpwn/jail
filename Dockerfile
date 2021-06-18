FROM debian:10.7-slim AS nsjail
WORKDIR /app
RUN apt-get update && apt-get install -y curl autoconf bison flex gcc g++ git libprotobuf-dev libnl-route-3-dev libtool make pkg-config protobuf-compiler
COPY nsjail .
RUN make

FROM golang:1.16.5-buster AS run
WORKDIR /app
RUN apt-get update && apt-get install -y protobuf-compiler libseccomp-dev libgmp-dev && go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.26.0
COPY go.mod go.sum .
RUN go mod download
COPY nsjail/config.proto nsjail/
COPY cmd cmd
RUN go generate ./cmd/jailrun && go build -v -ldflags "-w -s" ./cmd/jailrun

FROM busybox:1.32.1-glibc
RUN adduser -HDu 1000 nsjail && \
    mkdir -p /srv /jail/cgroup/cpu /jail/cgroup/mem /jail/cgroup/pids /jail/cgroup/unified /jail/dev && \
    mknod -m 666 /jail/dev/null c 1 3 && \
    mknod -m 666 /jail/dev/zero c 1 5 && \
    mknod -m 444 /jail/dev/urandom c 1 9
COPY --from=run /usr/lib/x86_64-linux-gnu/libseccomp.so.2 /usr/lib/x86_64-linux-gnu/libgmp.so.10 /lib/
COPY --from=nsjail /usr/lib/x86_64-linux-gnu/libprotobuf.so.17 \
    /usr/lib/x86_64-linux-gnu/libnl-route-3.so.200 \
    /lib/x86_64-linux-gnu/libnl-3.so.200 \
    /lib/x86_64-linux-gnu/libz.so.1 \
    /usr/lib/x86_64-linux-gnu/libstdc++.so.6 \
    /lib/x86_64-linux-gnu/libgcc_s.so.1 \
    /lib/
COPY --from=nsjail /app/nsjail /jail/nsjail
COPY --from=run /app/jailrun /jail/run
CMD ["/jail/run"]
