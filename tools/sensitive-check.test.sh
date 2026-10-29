#!/usr/bin/env bash
# sensitive-check.sh 시험. 가짜 값은 조각을 이어 붙여 만든다 — 이 파일 자체가 --tree 검사에 걸리지 않게 하려는 것이다.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
CHECK="$HERE/sensitive-check.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0; fail=0
ok()  { pass=$((pass + 1)); echo "ok   - $1"; }
bad() { fail=$((fail + 1)); echo "FAIL - $1"; }
expect_contains() { if grep -qF -- "$2" <<<"$3"; then ok "$1"; else bad "$1 (없음: $2)"; fi; }
expect_absent()   { if grep -qF -- "$2" <<<"$3"; then bad "$1 (있으면 안 됨: $2)"; else ok "$1"; fi; }

# 가짜 값 (리터럴로 붙어 있지 않다)
AWS="AKIA""IOSFODNN7EXAMPLE"
PW="pass""word=xxx"
IP10="10.""1.2.3"
IP192="192.""168.0.7"
IP172="172.""20.1.9"
MAIL="someone""@corp-example.io"
DENY_EN="Acme""Corp"
DENY_KO="오케""스트로"

export SENSITIVE_DENY_FILE="$TMP/deny.txt"
printf '# 시험용 deny\n%s\n%s\n' "$DENY_EN" "$DENY_KO" >"$SENSITIVE_DENY_FILE"

# ---- 1. --tree: 심은 값이 잡힌다 -------------------------------------
T="$TMP/tree"; mkdir -p "$T/sub"
{
    echo "key = $AWS"
    echo "$PW"
    echo "host $IP10 and $IP192 and $IP172"
    echo "mail $MAIL"
    echo "회사는 $(echo "$DENY_EN" | tr 'A-Z' 'a-z') 이다"      # 소문자 → 대소문자 무시 확인
    echo "우리는 ${DENY_KO}의 고객이다"                          # 한글 UTF-8 매칭
} >"$T/sub/notes.txt"
set +e; out="$("$CHECK" --tree "$T" 2>"$TMP/err")"; rc=$?; set -e
[ "$rc" = 1 ] && ok "tree: 종료 코드 1" || bad "tree: 종료 코드 $rc"
expect_contains "tree: AWS 키"        "sub/notes.txt:1 aws-access-key" "$out"
expect_contains "tree: 비밀번호 키-값"  "sub/notes.txt:2 credential-assign" "$out"
expect_contains "tree: 10.x IP"       "private-ip-10" "$out"
expect_contains "tree: 192.168 IP"    "private-ip-192" "$out"
expect_contains "tree: 172.16/12 IP"  "private-ip-172" "$out"
expect_contains "tree: 이메일"        "sub/notes.txt:4 email" "$out"
expect_contains "tree: deny 영문(대소문자 무시)" "sub/notes.txt:5 deny#2" "$out"
expect_contains "tree: deny 한글"     "sub/notes.txt:6 deny#3" "$out"

# ---- 2. 값은 앞 4자만 보인다 ------------------------------------------
expect_contains "마스킹: 앞 4자 + ****" "aws-access-key AKIA****" "$out"
expect_absent   "마스킹: AWS 키 원문이 없다" "$AWS" "$out"
expect_absent   "마스킹: 이메일 원문이 없다" "$MAIL" "$out"

# ---- 3. .sensitive-allow 로 빠진다 ------------------------------------
printf '# 예제 주소\n^%s$\n^%s$\n' "$(sed 's/\./\\./g' <<<"$IP10")" "$(sed 's/[.@]/\\&/g' <<<"$MAIL")" >"$T/.sensitive-allow"
set +e; out2="$("$CHECK" --tree "$T" 2>/dev/null)"; set -e
expect_absent   "allow: 허용한 IP 는 빠진다"    "private-ip-10" "$out2"
expect_absent   "allow: 허용한 이메일은 빠진다" "sub/notes.txt:4 email" "$out2"
expect_contains "allow: 허용 안 한 값은 그대로" "aws-access-key" "$out2"

# ---- 4. 깨끗한 트리는 0, 자리표시자는 값이 아니다 ---------------------
C="$TMP/clean"; mkdir -p "$C"
{
    echo 'Authorization: Bearer $TOKEN'
    echo 'password: <your-password>'
    echo 'token = next_token()'
    echo 'url: https://github.com/foo/bar'
} >"$C/ok.md"
set +e; "$CHECK" --tree "$C" >"$TMP/clean.out" 2>/dev/null; rc=$?; set -e
[ "$rc" = 0 ] && ok "clean: 종료 코드 0" || { bad "clean: 종료 코드 $rc"; cat "$TMP/clean.out"; }

# ---- 5. --staged -------------------------------------------------------
R="$TMP/repo"; mkdir -p "$R"
git -C "$R" init -q
git -C "$R" config user.name tester
git -C "$R" config user.email "tester""@dev.invalid"
echo "hello" >"$R/clean.txt"
git -C "$R" add clean.txt
set +e; (cd "$R" && "$CHECK" --staged >/dev/null 2>&1); rc=$?; set -e
[ "$rc" = 0 ] && ok "staged: 깨끗하면 0" || bad "staged: 깨끗한데 $rc"
echo "key = $AWS" >"$R/leak.txt"
git -C "$R" add leak.txt
set +e; out="$(cd "$R" && "$CHECK" --staged 2>/dev/null)"; rc=$?; set -e
[ "$rc" = 1 ] && ok "staged: 심으면 1" || bad "staged: 심었는데 $rc"
expect_contains "staged: 파일:줄" "leak.txt:1 aws-access-key" "$out"
git -C "$R" commit -qm "add clean and leak"

# ---- 6. --history: 지운 뒤에도 이력에 남은 값이 잡힌다 -----------------
git -C "$R" rm -q leak.txt
echo "$DENY_EN 내부 메모" >"$R/memo.txt"
git -C "$R" add memo.txt
git -C "$R" commit -qm "remove leak, note $(echo "$DENY_EN" | tr 'A-Z' 'a-z')"
set +e; out="$("$CHECK" --history "$R" 2>/dev/null)"; rc=$?; set -e
[ "$rc" = 1 ] && ok "history: 종료 코드 1" || bad "history: 종료 코드 $rc"
expect_contains "history: 지워진 키가 옛 커밋에서 잡힌다" "leak.txt:1 aws-access-key" "$out"
expect_contains "history: 커밋 메시지의 deny 단어"        "(msg)" "$out"
set +e; out="$("$CHECK" --tree "$R" 2>/dev/null)"; set -e
expect_absent   "tree: 지운 키는 작업 트리에 없다" "aws-access-key" "$out"

# ---- 7. author/committer 이메일은 deny 로 잡힌다 ----------------------
git -C "$R" config user.email "dev@$(echo "$DENY_EN" | tr 'A-Z' 'a-z').example"
echo more >>"$R/clean.txt"
git -C "$R" commit -qam "touch"
set +e; out="$("$CHECK" --history "$R" 2>/dev/null)"; set -e
expect_contains "history: author 이메일" "(author):0 deny#2" "$out"

# ---- 8. deny 파일이 없으면 경고하고 §11.1 만 쓴다 ---------------------
set +e; out="$(SENSITIVE_DENY_FILE="$TMP/없음.txt" "$CHECK" --tree "$T" 2>"$TMP/err")"; rc=$?; set -e
expect_contains "no-deny: 경고를 낸다"        "deny 파일이 없다" "$(cat "$TMP/err")"
expect_contains "no-deny: §11.1 패턴은 동작"  "aws-access-key" "$out"
expect_absent   "no-deny: deny 단어는 건너뜀" "deny#" "$out"

# ---- 9. 사용법 오류 ----------------------------------------------------
set +e; "$CHECK" >/dev/null 2>&1; rc=$?; set -e
[ "$rc" = 2 ] && ok "usage: 인자 없으면 2" || bad "usage: $rc"

echo
echo "통과 $pass, 실패 $fail"
[ "$fail" = 0 ]
