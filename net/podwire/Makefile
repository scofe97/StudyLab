# podwire — 빌드와 실습 진입점
#
# 개발은 macOS 에서 하지만 실행은 리눅스다. netns·veth·iptables·eBPF 는
# 전부 리눅스 커널 기능이라 OrbStack VM 이나 kind 노드 안에서만 돈다.
# 그래서 build 는 늘 크로스 컴파일이다.

GOOS_LINUX ?= linux
GOARCH ?= arm64
BIN := bin
VM ?= ubuntu

.PHONY: all build test vet fmt clean vm-push kind-up kind-down install help

all: vet test build

## build: 리눅스 arm64 바이너리 셋을 bin/ 에 만든다
build:
	@mkdir -p $(BIN)
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH) go build -o $(BIN)/pw ./cmd/pw
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH) go build -o $(BIN)/podwire ./cmd/podwire
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH) go build -o $(BIN)/podwire-agent ./cmd/podwire-agent
	@ls -lh $(BIN)

## test: 단위 테스트. 커널을 건드리지 않는 것만 여기서 돈다
test:
	go test ./...

## vet: 정적 검사
vet:
	go vet ./...

## fmt: 포맷
fmt:
	gofmt -w .

## vm-push: OrbStack VM 으로 바이너리를 보낸다 (Stage 0·1 실습용)
vm-push: build
	orb -m $(VM) mkdir -p /home/$$USER/podwire-bin
	cp $(BIN)/pw $(BIN)/podwire /Users/$$USER/podwire-bin-stage 2>/dev/null || true
	@echo "OrbStack 은 홈을 공유하므로 VM 안에서 $(CURDIR)/$(BIN) 을 그대로 쓸 수 있다"
	orb -m $(VM) ls -l $(CURDIR)/$(BIN)

## kind-up: 기본 CNI 없는 3노드 클러스터를 띄운다
kind-up:
	kind create cluster --config deploy/kind-cluster.yaml

## kind-down: 클러스터를 지운다
kind-down:
	kind delete cluster --name podwire

## install: 빌드한 플러그인을 kind 노드마다 배치한다
install: build
	./deploy/install.sh

## help: 타깃 목록
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
