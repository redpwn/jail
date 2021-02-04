FROM debian:10.7-slim AS build

WORKDIR /jail
RUN apt-get update && \
    apt-get install -y curl autoconf bison flex gcc g++ git libprotobuf-dev libnl-route-3-dev libtool make pkg-config protobuf-compiler
RUN git clone https://github.com/google/nsjail . && git checkout e1e80e8efa902f4c01c4bb39aa72af99c97b8a37 && make

FROM busybox:1.32.1-glibc

RUN adduser -HDu 1000 nsjail && \
    mkdir -p /app /jail/cgroup/cpu,cpuacct /jail/cgroup/memory /jail/cgroup/pids
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
