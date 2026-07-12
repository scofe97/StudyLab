package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
)

// 인메모리 순차 ID 카운터.
// Pod가 여러 개면 각 Pod가 독립 카운터를 가지므로, /id 응답의 Pod 이름과 함께 보면
// 어느 Pod가 응답했는지 + 그 Pod의 로컬 카운터 값을 확인할 수 있다.
// (5장 무중단 배포·6장 Valkey 상태 공유의 "왜 상태를 밖으로 빼야 하나"를 체감하는 장치)
var counter atomic.Int64

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

	// 포트 8080 — Dockerfile EXPOSE·Service targetPort와 일치해야 한다.
	log.Println("notiflex-api listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
