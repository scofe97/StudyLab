#!/usr/bin/env bash
# 01-04 패킷 캡처 실습 — 사전 점검
#
# 이 실습은 "에러 없이 틀린 결과"가 나오기 쉽다. 8080 이 남에게 점유돼 있으면
# curl 이 성공하고 패킷도 잡히는데 12 가 아니고, example.com 이 hosts 에
# 잡혀 있으면 캡처가 조용히 비어 있다. 그 두 함정을 여기서 먼저 막는다.
#
# FAIL 이 하나라도 있으면 exit 1. 조용히 넘기지 않는다.

cd "$(dirname "$0")" || exit 1
# shellcheck source=env.sh
source ./env.sh

fail=0
pass() { printf '  \033[32mPASS\033[0m  %s\n' "$1"; }
warn() { printf '  \033[33mWARN\033[0m  %s\n' "$1"; }
bad()  { printf '  \033[31mFAIL\033[0m  %s\n' "$1"; fail=$((fail+1)); }

echo "== 1. 도구 =="
for c in go tcpdump curl nc dig; do
  if command -v "$c" >/dev/null 2>&1; then pass "$c"; else bad "$c 없음"; fi
done
command -v vhs >/dev/null 2>&1 && pass "vhs (tape 굽기용, 선택)" || warn "vhs 없음 — tape 굽기만 불가"

echo
echo "== 2. 포트 (문서는 8080, 이 기기는 Docker 가 점유) =="
for p in "$LAB_PORT" "$LAB_CW_PORT"; do
  holder=$(lsof -nP -iTCP:"$p" -sTCP:LISTEN -F c 2>/dev/null | sed -n 's/^c//p' | head -1)
  if [ -z "$holder" ]; then
    pass "$p 비어 있음"
  else
    bad "$p 를 '$holder' 가 점유 중 — 이 포트로 실습하면 12 패킷이 재현되지 않는다"
  fi
done

echo
echo "== 3. §5 TLS 대상 도메인 =="
if grep -qE "^[^#]*[[:space:]]${LAB_TLS_HOST}([[:space:]]|$)" /etc/hosts; then
  bad "/etc/hosts 가 ${LAB_TLS_HOST} 를 오버라이드 중 — §5 캡처가 비어 나온다. 그 토큰을 지우고 다시 실행"
else
  pass "/etc/hosts 에 ${LAB_TLS_HOST} 오버라이드 없음"
  ip=$(dig +short "$LAB_TLS_HOST" 2>/dev/null | grep -E '^[0-9]+\.' | head -1)
  case "$ip" in
    ""|127.*) bad "${LAB_TLS_HOST} 가 공인 IP 로 안 풀림 (got: ${ip:-none})" ;;
    *)        pass "${LAB_TLS_HOST} -> $ip (공인)" ;;
  esac
fi

echo
echo "== 4. §1 관측 지점 사실 (문서의 출발점) =="
lo_eth=$(ifconfig lo0 2>/dev/null | grep -cE '^[[:space:]]*ether')
en_eth=$(ifconfig en0 2>/dev/null | grep -cE '^[[:space:]]*ether')
[ "$lo_eth" -eq 0 ] && pass "lo0 ether 0 개 (MAC 없음 → ARP 관측 불가)" || bad "lo0 ether 가 $lo_eth 개 — 문서 전제와 다름"
[ "$en_eth" -eq 1 ] && pass "en0 ether 1 개 (MAC 있음 → §3 ARP 가능)" || bad "en0 ether 가 $en_eth 개 — 문서 전제와 다름"
echo "        lo0 $(ifconfig lo0 | grep -o 'mtu [0-9]*')  /  en0 $(ifconfig en0 | grep -o 'mtu [0-9]*')"

echo
echo "== 5. §3 ARP 대상 (en0 기본 게이트웨이) =="
gw=$(netstat -rn -f inet | awk '$1=="default" && $NF=="en0"{print $2; exit}')
if [ -z "$gw" ]; then
  bad "en0 기본 게이트웨이 없음 — §3 을 돌릴 대상이 없다"
else
  pass "게이트웨이 확인 (주소는 민감정보라 출력하지 않음)"
  if ping -c 1 -t 2 "$gw" >/dev/null 2>&1; then pass "게이트웨이 ping 응답 — §3 대상으로 적합"; else bad "게이트웨이 무응답"; fi
fi
vpn=$(ifconfig 2>/dev/null | grep -cE '^utun[0-9]+.*UP')
[ "$vpn" -gt 0 ] && warn "VPN utun 인터페이스 ${vpn} 개 활성 — 캡처는 en0 로 지정할 것 (§3·§5 경로는 en0 로 실측 확인됨)"

echo
echo "== 6. 캡처 권한 =="
if sudo -n true 2>/dev/null; then
  pass "sudo 자격 증명 캐시됨 — 라이브 캡처 바로 가능"
else
  warn "sudo 미캐시 — 라이브 캡처(§2·§3·§5) 전에 'sudo -v' 를 먼저 실행"
fi

echo
if [ "$fail" -gt 0 ]; then
  printf '\033[31m%d 건 FAIL — 위 항목을 고친 뒤 실습을 시작한다.\033[0m\n' "$fail"
  exit 1
fi
printf '\033[32m전 항목 PASS — README.md 의 절 순서대로 진행한다.\033[0m\n'
