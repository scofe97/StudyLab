package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
)

// 인메모리 순차 ID 카운터.
// Pod가 여러 개면 각 Pod가 독립 카운터를 가지므로, /id 응답의 Pod 이름과 함께 보면
// 어느 Pod가 응답했는지 + 그 Pod의 로컬 카운터 값을 확인할 수 있다.
// (5장 무중단 배포·6장 Valkey 상태 공유의 "왜 상태를 밖으로 빼야 하나"를 체감하는 장치)
var counter atomic.Int64

// 앱 버전 — 롤링 업데이트로 배포될 때마다 올린다. 매니페스트 이미지 태그와 짝을 맞춘다.
const appVersion = "v0.1.1"

func main() {
	// GET /health — 헬스체크용. readiness/liveness probe가 이 경로를 찌른다.
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// GET /id — 순차 ID 생성 + 응답한 Pod 이름 반환.
	// HOSTNAME 환경변수는 쿠버네티스가 Pod 이름으로 주입한다(다운워드 API 없이도 기본 제공).
	http.HandleFunc("/id", func(w http.ResponseWriter, r *http.Request) {
		id := counter.Add(1)
		pod := os.Getenv("HOSTNAME") // k8s가 Pod 이름을 HOSTNAME으로 넣어줌
		if pod == "" {
			pod = "local"
		}
		fmt.Fprintf(w, "id=%d pod=%s\n", id, pod)
	})

	// GET /version — 앱 버전·Go 런타임 버전·응답한 Pod 이름을 JSON으로 반환.
	// 롤링 업데이트 중 어느 버전의 Pod가 응답하는지 눈으로 확인하는 용도(3.3절).
	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		host := os.Getenv("HOSTNAME") // k8s가 Pod 이름을 HOSTNAME으로 넣어줌 (/id와 동일)
		if host == "" {
			host = "local"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"version\":\"%s\",\"go_version\":\"%s\",\"hostname\":\"%s\"}\n",
			appVersion, runtime.Version(), host)
	})

	// 포트 8080 — Dockerfile EXPOSE·Service targetPort와 일치해야 한다.
	log.Println("notiflex-api listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
