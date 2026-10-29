# 5장은 DHCP·DNS·HTTP 를 한 이미지로 돌린다. 서버는 dnsmasq(DHCP+DNS 겸용), 클라이언트는 busybox udhcpc.
FROM ubuntu:24.04

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      dnsmasq udhcpc isc-dhcp-client iproute2 iputils-ping iputils-arping tcpdump \
      dnsutils curl python3 tinyproxy openssl ca-certificates \
      avahi-daemon avahi-utils libnss-mdns dbus procps \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /lab
