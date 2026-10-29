# 4장은 서버의 버전·cipher suite 를 마음대로 못박아야 해서 openssl s_server 를 서버로 쓴다.
FROM ubuntu:24.04

RUN apt-get update \
 && apt-get install -y --no-install-recommends openssl curl tcpdump iproute2 ca-certificates \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /lab
