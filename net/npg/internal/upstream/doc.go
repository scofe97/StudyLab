// Package upstream 은 npgproxy 가 업스트림으로 나가는 연결 층입니다.
//
// v0 에서는 비어 있고 Go 기본 Transport 를 그대로 씁니다. 장애 리허설을 겪은 뒤
// 기한·연결 풀(ADR 0012)과 재시도·멱등성(ADR 0013)을 이 패키지에 더합니다.
package upstream
