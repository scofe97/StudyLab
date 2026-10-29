# 원서 3장은 리눅스(CentOS)를 전제한다. ss·iptables·tc netem·sysctl 을 원서 명령 그대로 쓰려고 리눅스 컨테이너에서 돌린다.
FROM eclipse-temurin:17-jdk

RUN apt-get update \
 && apt-get install -y --no-install-recommends iproute2 iptables tcpdump net-tools procps \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /lab
