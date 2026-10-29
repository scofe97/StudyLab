.PHONY: help build build-linux test vet fmt lint check cover clean

.DEFAULT_GOAL := help

BIN := bin/gonet
BIN_LINUX := bin/gonet-linux
GO_BUILD_FLAGS := -trimpath -ldflags "-s -w"

help:
	@echo "targets:"
	@echo "  make build        - gonet 바이너리 빌드 (macOS)"
	@echo "  make build-linux  - OrbStack ubuntu(arm64)용 바이너리 빌드"
	@echo "  make test         - go test -race"
	@echo "  make vet          - go vet"
	@echo "  make fmt          - gofmt 적용"
	@echo "  make lint         - gofmt 미적용 파일 검출"
	@echo "  make check        - vet + lint + test (커밋 전)"
	@echo "  make cover        - 커버리지 요약"

build:
	CGO_ENABLED=0 go build $(GO_BUILD_FLAGS) -o $(BIN) ./cmd/gonet

build-linux:
	@# strace·ss·tc netem·eBPF 관측은 Linux 에서만 된다. OrbStack 머신은 ~ 를 공유하므로
	@# 빌드한 파일을 그대로 orb -m ubuntu ./bin/gonet-linux 로 실행한다.
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(GO_BUILD_FLAGS) -o $(BIN_LINUX) ./cmd/gonet

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then echo "gofmt 필요:"; echo "$$out"; exit 1; fi
	@echo "gofmt: clean"

cover:
	go test -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

check: vet lint test

clean:
	rm -rf bin/ coverage.out
