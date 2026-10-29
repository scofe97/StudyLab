#!/usr/bin/env bash
# 자체 CA 하나와 그 CA 가 서명한 서버 인증서 둘을 만든다.
# 두 장인 이유는 SNI 실험 때문이다 — 같은 IP·포트에서 이름에 따라 다른 인증서가 나가는 것을 본다.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p certs
cd certs

openssl req -x509 -newkey rsa:2048 -nodes -keyout ca.key -out ca.crt -days 365 \
  -subj "/CN=paw-lab Root CA" 2>/dev/null

for host in a.test b.test; do
  openssl req -newkey rsa:2048 -nodes -keyout "$host.key" -out "$host.csr" \
    -subj "/CN=$host" 2>/dev/null
  openssl x509 -req -in "$host.csr" -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out "$host.crt" -days 365 -extfile <(printf "subjectAltName=DNS:%s" "$host") 2>/dev/null
done

rm -f ./*.csr ./*.srl
ls -1
