#!/usr/bin/env bash
# BGP 실습 검증 — eBGP 와 iBGP 가 무엇이 다른지 출력으로 확인한다.
set -uo pipefail
cd "$(dirname "$0")"
fail=0
pass() { printf '  \033[32mPASS\033[0m  %s\n' "$1"; }
bad()  { printf '  \033[31mFAIL\033[0m  %s\n' "$1"; fail=$((fail+1)); }

for r in r1 r2 r3; do
  docker ps --format '{{.Names}}' | grep -qx "$r" || { echo "$r 미기동 — 'bash up.sh' 먼저"; exit 1; }
done

echo "== 1. 세션 수립 =="
sum=$(docker exec r1 vtysh -c 'show bgp summary' 2>/dev/null)
# State/PfxRcd 가 숫자면 Established, 문자면 미수립
for peer in 172.30.0.3:iBGP 172.30.0.4:eBGP; do
  ip=${peer%%:*}; kind=${peer##*:}
  st=$(echo "$sum" | awk -v ip="$ip" '$1==ip{print $10}')
  case "$st" in
    ''|*[!0-9]*) bad "$kind ($ip) 미수립 — state=${st:-없음}" ;;
    *)           pass "$kind ($ip) Established, 접두사 $st 개 수신" ;;
  esac
done

echo
echo "== 2. eBGP 대 iBGP — 같은 정보를 어떻게 다르게 싣는가 =="
e=$(docker exec r1 vtysh -c 'show bgp ipv4 unicast 10.0.0.3/32' 2>/dev/null)
i=$(docker exec r1 vtysh -c 'show bgp ipv4 unicast 10.0.0.2/32' 2>/dev/null)

echo "$e" | grep -q 'external' && pass "eBGP 경로가 'external' 로 표시" || bad "eBGP 경로에 external 없음"
echo "$e" | grep -qE '^\s+65002$' && pass "eBGP 경로의 AS_PATH 에 65002 가 붙음 (AS 를 건너며 추가)" || bad "AS_PATH 에 65002 없음"
echo "$i" | grep -q 'internal' && pass "iBGP 경로가 'internal' 로 표시" || bad "iBGP 경로에 internal 없음"
echo "$i" | grep -q 'Local'    && pass "iBGP 경로의 AS_PATH 가 비어 있음 (같은 AS 라 안 붙음)" || bad "iBGP AS_PATH 가 Local 이 아님"
echo "$i" | grep -q 'localpref 100' && pass "iBGP 경로에만 localpref 100 (내부 선호도)" || bad "iBGP localpref 없음"
echo "$e" | grep -q 'localpref' && bad "eBGP 경로에 localpref 가 붙음 (예상 밖)" || pass "eBGP 경로엔 localpref 없음"

echo
echo "== 3. iBGP 전파와 next-hop 함정 =="
r2t=$(docker exec r2 vtysh -c 'show bgp ipv4 unicast 10.0.0.3/32' 2>/dev/null)
if echo "$r2t" | grep -q '65002'; then
  pass "r2 가 r3(AS65002) 경로를 r1 을 통해 학습"
  if echo "$r2t" | grep -q 'from 172.30.0.2'; then
    pass "학습 출처는 r1(172.30.0.2)"
  else bad "출처가 r1 이 아님"; fi
  if echo "$r2t" | grep -q '172.30.0.4 from'; then
    pass "그런데 next-hop 은 r3(172.30.0.4) 그대로 ← eBGP→iBGP 시 next-hop 불변"
  else bad "next-hop 이 예상과 다름"; fi
else
  bad "r2 에 65002 경로 없음"
fi

echo
echo "== 4. 실제 도달성 =="
if docker exec r2 ping -c 1 -W 2 10.0.0.3 >/dev/null 2>&1; then
  pass "r2 -> r3 loopback 통신 성공"
else
  bad "r2 -> r3 통신 실패 (next-hop 도달 불가일 수 있음)"
fi

echo
[ "$fail" -gt 0 ] && { printf '\033[31m%d 건 FAIL\033[0m\n' "$fail"; exit 1; }
printf '\033[32m전 항목 PASS\033[0m — capture.sh 로 179 포트를 직접 들여다보세요.\n'
