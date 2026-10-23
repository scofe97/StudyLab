#!/usr/bin/env bash
# 장애 리허설에 쓸 파일을 work/tftp/ 아래에 만든다. 여러 번 돌려도 된다 (이미 있으면 건너뛴다).
#
#   work/tftp/root/      서버 루트 (tftpd -root work/tftp/root)
#   work/tftp/outside/   루트 밖 — T6 에서 열리면 안 되는 파일
#   work/tftp/dl/        클라이언트가 받은 파일을 둘 곳
#
# 사용: 저장소 루트에서 ./scripts/tftp-mkroot.sh
set -euo pipefail
cd "$(dirname "$0")/.."

base=work/tftp
mkdir -p $base/root/sub $base/outside $base/dl

# mkrand <이름> <바이트 수> : 무작위 바이트 파일
mkrand() {
  local path="$base/root/$1" size="$2"
  if [[ -f "$path" && "$(wc -c <"$path" | tr -d ' ')" == "$size" ]]; then
    return
  fi
  head -c "$size" /dev/urandom >"$path"
  echo "made $path ($size bytes)"
}

# T3 지사 부팅 파일 (128 KiB), T4 중복 ACK 장면 (1 MiB = 512바이트 블록 2048개), T5 측정용 (32 KiB = 64블록)
mkrand boot-128k.bin 131072
mkrand fw.bin 1048576
mkrand boot-32k.bin 32768

# T6 루트 밖 파일과 루트 안 링크·디렉터리
[[ -f $base/outside/secret.txt ]] || echo "OUTSIDE-SECRET-DO-NOT-SERVE" >$base/outside/secret.txt
ln -sfn ../outside/secret.txt $base/root/link-out
echo "inside" >$base/root/sub/in.txt

ls -l $base/root | cat
