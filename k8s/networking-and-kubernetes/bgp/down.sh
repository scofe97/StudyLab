#!/usr/bin/env bash
# 실습 정리 — 컨테이너와 네트워크만 제거. kind 클러스터·Jenkins 는 건드리지 않는다.
docker rm -f r1 r2 r3 >/dev/null 2>&1 || true
docker network rm bgplab >/dev/null 2>&1 || true
echo "정리 완료 (kind k8s-lab·sq-jenkins 무영향)"
