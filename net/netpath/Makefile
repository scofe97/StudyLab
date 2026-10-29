.PHONY: help build test vet fmt lint check cover vuln clean

.DEFAULT_GOAL := help

BIN := bin/netpath
GO_BUILD_FLAGS := -trimpath -ldflags "-s -w"

help:
	@echo "targets:"
	@echo "  make build   - netpath 바이너리 빌드"
	@echo "  make test    - go test -race"
	@echo "  make vet     - go vet"
	@echo "  make fmt     - gofmt 적용"
	@echo "  make lint    - gofmt 미적용 파일 검출 (CI용)"
	@echo "  make check   - vet + lint + test (커밋 전)"
	@echo "  make cover   - 커버리지 요약"
	@echo "  make vuln    - govulncheck (네트워크 필요)"

build:
	@# CGO_ENABLED=0 으로 정적 바이너리를 만든다. 3국면에서 distroless 이미지에 넣으려면
	@# 동적 링크가 없어야 하고, DNS 도 순수 Go 리졸버 경로로 고정된다.
	@# test 에는 걸지 않는다 — -race 가 플랫폼에 따라 cgo 를 요구할 수 있다.
	CGO_ENABLED=0 go build $(GO_BUILD_FLAGS) -o $(BIN) ./cmd/netpath

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
	@# 코어 경계 검사: probe 는 클러스터도 관측 백엔드도 몰라야 한다.
	@deps="$$(go list -deps ./probe 2>/dev/null | grep -E 'k8s\.io|go\.opentelemetry\.io' || true)"; \
	if [ -n "$$deps" ]; then echo "probe 코어에 외부 의존성 유입:"; echo "$$deps"; exit 1; fi
	@echo "probe core: clean"

cover:
	go test -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

vuln:
	@# 외부 의존이 0개여도 표준 라이브러리의 CVE 를 잡는다.
	@# 네트워크가 필요하므로 check 에는 넣지 않는다.
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

check: vet lint test

clean:
	rm -rf bin/ coverage.out
