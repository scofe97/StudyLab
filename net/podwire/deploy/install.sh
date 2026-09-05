#!/usr/bin/env bash
# kind 노드마다 podwire 플러그인과 설정을 배치한다.
#
# extraMounts 대신 docker cp 를 쓴다. 마운트는 호스트 경로 해석이 환경마다 달라
# OrbStack 에서 되는지 확인이 필요한 항목인데, docker cp 는 어디서나 같게 동작한다.
# 빌드할 때마다 다시 실행하면 되므로 개발 루프에도 지장이 없다.
#
# podCIDR 는 노드마다 다르다. API 에서 읽어 노드별 설정을 따로 만든다 —
# 한 값을 모든 노드에 복사하면 두 노드가 같은 대역에서 주소를 내주고,
# 서로 다른 Pod 이 같은 IP 를 갖게 된다.
set -euo pipefail

CLUSTER="${CLUSTER:-podwire}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$REPO_ROOT/bin/podwire"
CONF_TMPL="$REPO_ROOT/deploy/10-podwire.conflist"

[[ -x "$BIN" ]] || { echo "먼저 make build 를 실행하세요: $BIN 없음" >&2; exit 1; }

nodes="$(kind get nodes --name "$CLUSTER")"

for node in $nodes; do
  # 노드 이름은 kind 컨테이너 이름과 같다.
  k8s_node="$node"
  cidr="$(kubectl get node "$k8s_node" -o jsonpath='{.spec.podCIDR}')"
  if [[ -z "$cidr" ]]; then
    echo "$k8s_node: podCIDR 가 비어 있음 — 컨트롤러 매니저가 아직 할당하지 않았을 수 있습니다" >&2
    exit 1
  fi
  echo "== $k8s_node (podCIDR=$cidr)"

  tmp="$(mktemp)"
  sed "s|PODCIDR_PLACEHOLDER|$cidr|" "$CONF_TMPL" > "$tmp"

  docker exec "$node" mkdir -p /opt/cni/bin /etc/cni/net.d /var/lib/cni/podwire
  docker cp "$BIN" "$node:/opt/cni/bin/podwire"
  docker cp "$tmp" "$node:/etc/cni/net.d/10-podwire.conflist"
  docker exec "$node" chmod +x /opt/cni/bin/podwire
  rm -f "$tmp"
done

echo
echo "배치 완료. 확인:"
echo "  kubectl get pods -A -o wide"
echo "  docker exec ${CLUSTER}-control-plane journalctl -u kubelet -n 50 --no-pager | grep -i cni"
