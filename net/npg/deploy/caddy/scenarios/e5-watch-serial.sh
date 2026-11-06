#!/usr/bin/env bash
# 내부 CA 가 낸 잎 인증서의 serial 과 만료 시각을 일정 간격으로 찍는다.
#   scenarios/e5-watch-serial.sh [호스트] [간격초] [횟수]
host="${1:-short.corp.internal}" every="${2:-15}" count="${3:-14}"
for _ in $(seq 1 "$count"); do
  printf '%s  ' "$(date +%H:%M:%S)"
  openssl s_client -connect 127.0.0.1:8443 -servername "$host" </dev/null 2>/dev/null |
    openssl x509 -noout -serial -enddate | paste -sd' ' -
  sleep "$every"
done
