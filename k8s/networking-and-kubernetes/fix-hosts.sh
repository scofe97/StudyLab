#!/usr/bin/env bash
# /etc/hosts 에서 example.com 오버라이드만 제거한다.
#
# 왜: 7행이 `127.0.0.1  localhost example.com` 이라 example.com 이 루프백으로 잡힌다.
#     그러면 §5 의 `curl https://example.com` 이 즉시 실패하고 en0 캡처가 비어 나온다.
# 무엇을 안 건드리나: 회사 인프라 항목 331 행. localhost 토큰도 반드시 남긴다.
set -euo pipefail

if ! grep -qE '^[^#]*[[:space:]]example\.com([[:space:]]|$)' /etc/hosts; then
  echo "이미 제거돼 있음 — 할 일 없음"; exit 0
fi

LINE=$(grep -nE '^[^#]*[[:space:]]example\.com([[:space:]]|$)' /etc/hosts | cut -d: -f1)
COUNT=$(printf '%s\n' "$LINE" | wc -l | tr -d ' ')
if [ "$COUNT" -ne 1 ]; then
  echo "example.com 이 ${COUNT} 행에 있음 — 자동 수정 중단. 수동 확인 요망"; exit 1
fi

BAK="/etc/hosts.bak.$(date +%Y%m%d%H%M%S)"
sudo cp -p /etc/hosts "$BAK"
echo "백업: $BAK"
echo "수정 전 ${LINE}행: $(sed -n "${LINE}p" /etc/hosts)"

sudo sed -i '' "${LINE}s/[[:space:]]*example\.com//" /etc/hosts
echo "수정 후 ${LINE}행: $(sed -n "${LINE}p" /etc/hosts)"

# 검증 — localhost 가 살아 있어야 하고 example.com 은 사라져야 한다
sed -n "${LINE}p" /etc/hosts | grep -q localhost || { echo "localhost 유실! 즉시 복구: sudo cp $BAK /etc/hosts"; exit 1; }
grep -qE '^[^#]*[[:space:]]example\.com([[:space:]]|$)' /etc/hosts && { echo "제거 실패"; exit 1; }
echo "행 수: $(wc -l < /etc/hosts) (338 이어야 함)"
echo "OK — 이제 'bash preflight.sh' 가 전 항목 PASS 여야 합니다"
