FROM debian:10.7-slim AS build

WORKDIR /jail
RUN apt-get update && \
    apt-get install -y curl autoconf bison flex gcc g++ git libprotobuf-dev libnl-route-3-dev libtool make pkg-config protobuf-compiler
RUN git clone https://github.com/google/nsjail . && git checkout 645eabd862e4eb20ec70a387fb7d50ecbc613f6e && make

FROM busybox:1.32.1-glibc

RUN adduser -HDu 1000 nsjail && \
    mkdir -p /app /jail/cgroup/cpu /jail/cgroup/memory /jail/cgroup/pids /jail/dev && \
    mknod -m 666 /jail/dev/null c 1 3 && \
    mknod -m 666 /jail/dev/zero c 1 5 && \
    mknod -m 444 /jail/dev/urandom c 1 9
COPY --from=build /jail/nsjail /jail/nsjail
COPY --from=build /usr/lib/x86_64-linux-gnu/libprotobuf.so.17 \
    /usr/lib/x86_64-linux-gnu/libnl-route-3.so.200 \
    /lib/x86_64-linux-gnu/libnl-3.so.200 \
    /lib/x86_64-linux-gnu/libz.so.1 \
    /usr/lib/x86_64-linux-gnu/libstdc++.so.6 \
    /lib/x86_64-linux-gnu/libgcc_s.so.1 \
    /lib/
COPY run.sh /jail
CMD [ "/jail/run.sh" ]
