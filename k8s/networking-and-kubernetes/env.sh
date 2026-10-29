#!/usr/bin/env bash
# 01-04 패킷 캡처 실습 — 환경 변수 SSOT
#
# 문서(01-04.md)는 8080 을 쓰지만 이 기기는 Docker sq-jenkins 가 8080 을 점유한다.
# 실습 포트를 여기서 한 번만 정하고 스크립트·명령이 전부 이 값을 따른다.
# Jenkins 를 내리고 문서 그대로 8080 을 쓰고 싶으면 LAB_PORT 만 고치면 된다.

export LAB_PORT="${LAB_PORT:-8083}"        # §2 루프백 웹 서버
export LAB_CW_PORT="${LAB_CW_PORT:-9090}"  # §4 CLOSE-WAIT 관찰용 서버
export LAB_TLS_HOST="${LAB_TLS_HOST:-example.com}"  # §5 TLS SNI 관찰 대상
export LAB_DNS="${LAB_DNS:-8.8.8.8}"       # §5 UDP 질의용 공개 리졸버
export LAB_PCAP="${LAB_PCAP:-/tmp/loop.pcap}"
