# 멀티스테이지 빌드 — 빌더에서 컴파일하고, 결과 바이너리만 scratch로 복사
# 왜 멀티스테이지: 최종 이미지에 Go 툴체인·소스·캐시를 남기지 않아 이미지가 수 MB로 작아지고
# 공격 표면(셸·패키지 관리자)이 사라진다.
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
# CGO_ENABLED=0: C 라이브러리에 의존하지 않는 정적 바이너리로 빌드.
# 이게 scratch(빈 이미지)에서 동작하는 전제 — 동적 링크였다면 libc가 없어 실행 불가.
RUN CGO_ENABLED=0 GOOS=linux go build -o notiflex-api .

# scratch = OS도 셸도 없는 완전히 빈 이미지. 정적 바이너리 하나만 올려도 돈다.
FROM scratch
COPY --from=builder /app/notiflex-api /notiflex-api
EXPOSE 8080
ENTRYPOINT ["/notiflex-api"]
