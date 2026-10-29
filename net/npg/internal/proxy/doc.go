// Package proxy 는 npgproxy 의 요청 전달 층입니다.
//
// 들어온 요청을 업스트림 하나로 넘기고, 실패하면 원인 코드(envoy 응답 플래그 이름을 따름)와
// 상태 코드를 골라 돌려줍니다. 접근 로그와 expvar 카운터로 업스트림 시간과 실패 원인을 남깁니다.
//
// 요청 하나가 지나가는 길 (각 함수 주석의 [흐름 N] 이 이 번호입니다):
//
//	손님 ──▶ [1] accessLog ──▶ [2] Rewrite ──▶ [3] Transport ──▶ 업스트림
//	                                              │
//	           [5] accessLog ◀── [4a] 응답 복사 ◀──┤ 성공
//	                         ◀── [4b] ErrorHandler ◀┘ 실패
//
//	[1]  accessLog    빈 기록장(reqInfo)을 ctx 에 달고 requests·inflight 를 올린다.
//	     withDeadline [1] 과 [2] 사이에서 ctx 에 요청 전체 기한(ADR 0012, 15초)을 건다. 넘기면 [3] 이
//	                  멈추고 [4b] 가 UT 를 고른다.
//	[2]  Rewrite      주소·Host 를 업스트림으로 바꾸고, 손님이 보낸 X-Forwarded-* 를 버린 뒤
//	                  진짜 IP 를 적고, 연결 추적 훅(traceConn)을 단다.
//	[3]  Transport    풀에서 연결을 꺼내거나 새로 맺는다. 이때 GotConn 훅이 새 연결/재사용을
//	                  세고 기록장에 reused·idle 을 적는다.
//	[4a] 성공         업스트림 응답을 손님에게 흘려보낸다. statusRecorder 가 상태·바이트를 센다.
//	[4b] 실패         classify 가 원인 코드(UO·UT·UF·UC·DC)를 고르고, 기록장에 적고, 상태 코드를 쓴다.
//	                  UO 는 [3] 의 풀 줄이 넘쳐 보내지도 않은 것이다(503).
//	[5]  accessLog    기록장과 statusRecorder 를 읽어 로그 한 줄을 쓰고 reasons 를 올린다.
//
// 기록장 하나를 ctx 로 돌려 쓰는 까닭: [3]·[4b] 는 ReverseProxy 안쪽에서 불려
// [5] 에 값을 직접 넘길 길이 없다. ctx 는 요청을 따라 끝까지 가므로 거기에 단다.
package proxy
