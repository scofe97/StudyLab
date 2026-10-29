#!/usr/bin/env bash
# 시나리오에 쓸 파일을 work/ 아래에 만든다. 여러 번 돌려도 된다 (이미 있으면 건너뛴다).
#
#   work/root/      서버 루트 (tftpd -root work/root)
#   work/outside/   루트 밖 — T6 에서 열리면 안 되는 파일
#   work/dl/        클라이언트가 받은 파일을 둘 곳
#
# 사용: cd ~/study/runners-lab/net/npg/chapters/ch06-tftp && ./scripts/mkroot.sh
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p work/root/pxelinux.cfg work/outside work/dl

# mkrand <이름> <바이트 수> : 무작위 바이트 파일
mkrand() {
  local path="work/root/$1" size="$2"
  if [[ -f "$path" && "$(wc -c <"$path" | tr -d ' ')" == "$size" ]]; then
    return
  fi
  head -c "$size" /dev/urandom >"$path"
  echo "made $path ($size bytes)"
}

# T1 펌웨어 이미지 (1 MiB)
mkrand fw.bin 1048576
# T3·T7 부팅 파일 (128 KiB)
mkrand boot-128k.bin 131072
# T5 측정용 작은 파일 (32 KiB)
mkrand boot-32k.bin 32768
# T2 큰 이미지와 경계 두 개
mkrand big.img 40000000
mkrand edge-33553919.img 33553919
mkrand edge-33553920.img 33553920

# T1 텍스트 설정 파일 (스위치 설정 백업 흉내, 줄 끝은 LF)
cat >work/root/switch01.cfg <<'EOF'
hostname switch01
interface GigabitEthernet0/1
 description uplink
 switchport mode trunk
!
end
EOF

# PXE 부팅 메뉴 흉내 (하위 폴더 안의 파일)
cat >work/root/pxelinux.cfg/default <<'EOF'
DEFAULT linux
LABEL linux
  KERNEL vmlinuz
  APPEND initrd=initrd.img
EOF

# T6 루트 밖 비밀 파일과, 루트 안에서 그것을 가리키는 심볼릭 링크
echo "this file lives outside the TFTP root" >work/outside/secret.txt
ln -sfn ../outside/secret.txt work/root/link-out

echo "--- work/root"
ls -l work/root
echo "--- SHA512/256"
shasum -a 512256 work/root/fw.bin work/root/switch01.cfg
