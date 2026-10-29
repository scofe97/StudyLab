// Package upstream 은 npgproxy 가 업스트림으로 나가는 연결 층입니다.
//
// v0 는 Go 기본 Transport 를 그대로 썼습니다. 장애 리허설(I1·I3·I2)을 겪은 뒤 ADR 0012 로
// 전용 Transport 와 검색/나머지 격벽(New)을 두었고, 풀마다 칸 + 줄을 넘는 요청은
// 보내지 않고 ErrOverflow 로 돌려보냅니다(limit, 결정 6). 요청 전체 기한 RequestTimeout 은
// 프록시 쪽 감싸개(proxy.withDeadline)가 겁니다. 재시도·멱등성(ADR 0013)도 이 패키지에 더합니다.
package upstream
